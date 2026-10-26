package runners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

// newLocalSourceRepo creates a local (no-network) git repo containing the
// eg/compile package's example.1 fixture as its .eg module, so
// compileWorkload can clone+compile it without a VCS auth token or network
// access to a real forge.
func newLocalSourceRepo(t *testing.T) (uri, ref string) {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, fsx.CloneTree(
		t.Context(),
		filepath.Join(dir, ".eg"),
		filepath.Join("example.1", ".eg"),
		os.DirFS(filepath.Join("..", "compile", ".fixtures")),
	))

	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	w, err := repo.Worktree()
	require.NoError(t, err)

	_, err = w.Add(".")
	require.NoError(t, err)

	_, err = w.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "eg", Email: "eg@example.com"},
	})
	require.NoError(t, err)

	head, err := repo.Head()
	require.NoError(t, err)

	return dir, head.Name().Short()
}

func TestCompileWorkload(t *testing.T) {
	t.Run("compiles a local source ref and enqueues it into rundirs", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		uid := uuid.Must(uuid.NewV7())
		req := computeapi.EnqueuedDequeueResponse{
			Enqueued: &computeapi.Enqueued{
				VcsUri:    uri,
				VcsCommit: ref,
				Cores:     1,
			},
			AccessToken: "token",
		}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(uid, eg.EnvironFile, strings.NewReader(fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, uri, eg.EnvGitHeadRef, ref))))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.NoError(t, compileWorkload(t.Context(), vcsaccess, dir, rundirs))

		// nothing is left staged in rundirs.Downloading.
		dentries, err := os.ReadDir(rundirs.Downloading)
		require.NoError(t, err)
		require.Empty(t, dentries)

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Len(t, qentries, 1)
		require.Equal(t, Queued().Dirname(uid), qentries[0].Name())

		target := filepath.Join(rundirs.Queued, qentries[0].Name())
		require.FileExists(t, filepath.Join(target, "archive.tar.gz"))

		mencoded, err := os.ReadFile(filepath.Join(target, "metadata.json"))
		require.NoError(t, err)

		var resp computeapi.EnqueuedDequeueResponse
		require.NoError(t, json.Unmarshal(mencoded, &resp))
		require.Equal(t, uid.String(), resp.Enqueued.Id)
		require.NotEmpty(t, resp.Enqueued.Entry)
		require.Equal(t, ref, resp.Enqueued.VcsCommit)

		// only what the run stage consumes is handed off.
		tentries, err := os.ReadDir(target)
		require.NoError(t, err)
		names := make([]string, 0, len(tentries))
		for _, e := range tentries {
			names = append(names, e.Name())
		}
		require.ElementsMatch(t, []string{"metadata.json", "archive.tar.gz"}, names)

		// the outgoing workload environment travels in the archive, without
		// any credentials.
		archive, err := os.Open(filepath.Join(target, "archive.tar.gz"))
		require.NoError(t, err)
		defer archive.Close()
		unpacked := t.TempDir()
		require.NoError(t, tarx.Unpack(unpacked, archive))
		outgoing, err := os.ReadFile(filepath.Join(unpacked, eg.EnvironFile))
		require.NoError(t, err)
		require.NotContains(t, string(outgoing), "EG_GIT_AUTH_ACCESS_TOKEN")
	})

	t.Run("failed compiles are discarded and never reach rundirs.Queued", func(t *testing.T) {
		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		missing := filepath.Join(t.TempDir(), "does-not-exist")
		uid := uuid.Must(uuid.NewV7())
		req := computeapi.EnqueuedDequeueResponse{Enqueued: &computeapi.Enqueued{VcsUri: missing}, AccessToken: "token"}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(uid, eg.EnvironFile, strings.NewReader(fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, missing, eg.EnvGitHeadRef, "main"))))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.Error(t, compileWorkload(t.Context(), vcsaccess, dir, rundirs))

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Empty(t, qentries)
	})

	t.Run("exchanges the access token for git credentials before cloning", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		var exchanged []string
		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			if !strings.HasSuffix(r.URL.Path, "/r/vcsaccess/") {
				t.Errorf("unexpected vcs access request: %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
				return w.Result(), nil
			}

			exchanged = append(exchanged, r.Header.Get("Authorization"))
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		uid := uuid.Must(uuid.NewV7())
		req := computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{VcsUri: uri, VcsCommit: ref, Cores: 1},
			AccessToken: "token",
		}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(uid, eg.EnvironFile, strings.NewReader(fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, uri, eg.EnvGitHeadRef, ref))))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.NoError(t, compileWorkload(t.Context(), vcsaccess, dir, rundirs))
		require.Equal(t, []string{"BEARER token"}, exchanged)

		// the short-lived credentials are only needed to clone, they don't
		// travel with the workload.
		target := filepath.Join(rundirs.Queued, filepath.Base(dir))
		require.DirExists(t, target)
		require.NoFileExists(t, filepath.Join(target, "vcsaccess.token"))
	})

	t.Run("never writes the exchanged credentials to disk", func(t *testing.T) {
		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		var exchanged []string
		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			exchanged = append(exchanged, r.Header.Get("Authorization"))
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		// the clone fails, leaving the job directory in place to inspect.
		missing := filepath.Join(t.TempDir(), "does-not-exist")
		uid := uuid.Must(uuid.NewV7())
		req := computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{VcsUri: missing},
			AccessToken: "token",
		}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(uid, eg.EnvironFile, strings.NewReader(fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, missing, eg.EnvGitHeadRef, "main"))))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.Error(t, compileWorkload(t.Context(), vcsaccess, dir, rundirs))
		require.Equal(t, []string{"BEARER token"}, exchanged)
		require.NoFileExists(t, filepath.Join(dir, "vcsaccess.token"))
		require.NoFileExists(t, filepath.Join(dir, "src", "vcsaccess.token"))
	})

	t.Run("clones the head uri/ref from the submitted environ", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		repo, err := git.PlainOpen(uri)
		require.NoError(t, err)
		head, err := repo.Head()
		require.NoError(t, err)

		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		// mirrors what egmeta pushes: the canonical vcs uri and commit, which
		// are not directly cloneable, alongside the clone uri and ref.
		uid := uuid.Must(uuid.NewV7())
		req := computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{VcsUri: "git@github.com:example/nope.git", VcsCommit: head.Hash().String(), Cores: 1},
			AccessToken: "token",
		}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(uid, eg.EnvironFile, strings.NewReader(fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, uri, eg.EnvGitHeadRef, ref))))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.NoError(t, compileWorkload(t.Context(), vcsaccess, dir, rundirs))

		archive, err := os.Open(filepath.Join(rundirs.Queued, filepath.Base(dir), "archive.tar.gz"))
		require.NoError(t, err)
		defer archive.Close()
		unpacked := t.TempDir()
		require.NoError(t, tarx.Unpack(unpacked, archive))
		outgoing, err := os.ReadFile(filepath.Join(unpacked, eg.EnvironFile))
		require.NoError(t, err)
		require.Contains(t, string(outgoing), eg.EnvGitHeadURI+"="+uri)
	})

	t.Run("fails when the submitted environ lacks the head uri/ref", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		req := computeapi.EnqueuedDequeueResponse{Enqueued: &computeapi.Enqueued{VcsUri: uri, VcsCommit: ref, Cores: 1}}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(uid, eg.EnvironFile, strings.NewReader("")))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.Error(t, compileWorkload(t.Context(), http.DefaultClient, dir, rundirs))

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Empty(t, qentries)
	})

	t.Run("fails without an access token", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			t.Errorf("unexpected vcs access request: %s", r.URL.Path)
			return httptest.NewRecorder().Result(), nil
		})}

		uid := uuid.Must(uuid.NewV7())
		req := computeapi.EnqueuedDequeueResponse{Enqueued: &computeapi.Enqueued{VcsUri: uri, VcsCommit: ref, Cores: 1}}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(uid, eg.EnvironFile, strings.NewReader(fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, uri, eg.EnvGitHeadRef, ref))))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.Error(t, compileWorkload(t.Context(), vcsaccess, dir, rundirs))

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Empty(t, qentries)
	})
}

// controlplane stands in for egmeta's queue api: it records the status phases
// and reservations runners report, in the order received.
type controlplane struct {
	mu       sync.Mutex
	calls    []string
	reserved computeapi.EnqueuedDequeueResponse
}

func (t *controlplane) Calls() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.calls...)
}

// Client routes every request to the control plane regardless of host.
func (t *controlplane) Client(tt *testing.T) *http.Client {
	return &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		t.ServeHTTP(tt, w, r)
		return w.Result(), nil
	})}
}

func (t *controlplane) ServeHTTP(tt *testing.T, w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch {
	case strings.HasSuffix(r.URL.Path, "/c/q/status"):
		require.NoError(tt, r.ParseForm())
		require.Equal(tt, "candidate", r.FormValue("token"))
		t.calls = append(t.calls, r.FormValue("phase"))
		w.WriteHeader(http.StatusAccepted)
	case strings.HasSuffix(r.URL.Path, "/c/q/reserved"):
		require.NoError(tt, r.ParseMultipartForm(1<<20))
		require.Equal(tt, "candidate", r.FormValue("token"))
		require.NotEmpty(tt, r.FormValue("entry"))
		f, _, err := r.FormFile("archive")
		require.NoError(tt, err)
		require.NoError(tt, f.Close())
		t.calls = append(t.calls, "reserved")
		require.NoError(tt, json.NewEncoder(w).Encode(&t.reserved))
	default:
		tt.Errorf("unexpected control plane request: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

type roundtripper func(*http.Request) (*http.Response, error)

func (t roundtripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return t(r)
}

func TestCompileWorkloadReserved(t *testing.T) {
	spool := func(t *testing.T, compiledirs SpoolDirs, uid uuid.UUID, req *computeapi.EnqueuedDequeueResponse, environ string) string {
		encoded, err := json.Marshal(req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(uid, eg.EnvironFile, strings.NewReader(environ)))
		require.NoError(t, compiledirs.Download(uid, CandidateTokenFile, strings.NewReader("candidate")))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)
		return dir
	}

	t.Run("pushed workloads are reserved once compiled and report their progress", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		cp := &controlplane{
			reserved: computeapi.EnqueuedDequeueResponse{
				Enqueued:    &computeapi.Enqueued{Id: uid.String(), VcsUri: uri, VcsCommit: ref, Entry: "reserved.wasm"},
				AccessToken: "fresh",
			},
		}

		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		dir := spool(t, compiledirs, uid, &computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{Id: uid.String(), VcsUri: uri, VcsCommit: ref, Cores: 1},
			AccessToken: "token",
		}, fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, uri, eg.EnvGitHeadRef, ref))

		require.NoError(t, compileWorkloadReserved(t.Context(), vcsaccess, cp.Client(t), dir, rundirs))
		require.Equal(t, []string{StatusCompiling, "reserved", StatusQueued}, cp.Calls())

		target := filepath.Join(rundirs.Queued, filepath.Base(dir))
		mencoded, err := os.ReadFile(filepath.Join(target, "metadata.json"))
		require.NoError(t, err)

		// the workload is recorded as the control plane reserved it.
		var resp computeapi.EnqueuedDequeueResponse
		require.NoError(t, json.Unmarshal(mencoded, &resp))
		require.Equal(t, uid.String(), resp.Enqueued.Id)
		require.Equal(t, "reserved.wasm", resp.Enqueued.Entry)
		require.Equal(t, "fresh", resp.AccessToken)

		// the token travels with the workload so it can report running.
		require.Equal(t, "candidate", ReadCandidateToken(target))

		// only what the run stage consumes is handed off.
		tentries, err := os.ReadDir(target)
		require.NoError(t, err)
		names := make([]string, 0, len(tentries))
		for _, e := range tentries {
			names = append(names, e.Name())
		}
		require.ElementsMatch(t, []string{"metadata.json", "archive.tar.gz", CandidateTokenFile}, names)
	})

	t.Run("pushed workloads missing the head uri/ref report failure", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		cp := &controlplane{}

		dir := spool(t, compiledirs, uid, &computeapi.EnqueuedDequeueResponse{
			Enqueued: &computeapi.Enqueued{Id: uid.String(), VcsUri: uri, VcsCommit: ref, Cores: 1},
		}, "")

		require.Error(t, compileWorkloadReserved(t.Context(), http.DefaultClient, cp.Client(t), dir, rundirs))
		require.Equal(t, []string{StatusCompiling, StatusFailed}, cp.Calls())

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Empty(t, qentries)
	})

	t.Run("pushed workloads report failures", func(t *testing.T) {
		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		cp := &controlplane{}

		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		missing := filepath.Join(t.TempDir(), "does-not-exist")
		dir := spool(t, compiledirs, uid, &computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{Id: uid.String(), VcsUri: missing},
			AccessToken: "token",
		}, fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, missing, eg.EnvGitHeadRef, "main"))

		require.Error(t, compileWorkloadReserved(t.Context(), vcsaccess, cp.Client(t), dir, rundirs))
		require.Equal(t, []string{StatusCompiling, StatusFailed}, cp.Calls())

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Empty(t, qentries)
	})

	t.Run("without the control plane pushed workloads fail to reserve", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		uid := uuid.Must(uuid.NewV7())
		dir := spool(t, compiledirs, uid, &computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{VcsUri: uri, VcsCommit: ref, Cores: 1},
			AccessToken: "token",
		}, fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, uri, eg.EnvGitHeadRef, ref))

		require.Error(t, compileWorkload(t.Context(), vcsaccess, dir, rundirs))

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Empty(t, qentries)
	})

	t.Run("pushed workloads without an access token report failure", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		cp := &controlplane{}

		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			t.Errorf("unexpected vcs access request: %s", r.URL.Path)
			return httptest.NewRecorder().Result(), nil
		})}

		dir := spool(t, compiledirs, uid, &computeapi.EnqueuedDequeueResponse{
			Enqueued: &computeapi.Enqueued{Id: uid.String(), VcsUri: uri, VcsCommit: ref, Cores: 1},
		}, fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, uri, eg.EnvGitHeadRef, ref))

		require.Error(t, compileWorkloadReserved(t.Context(), vcsaccess, cp.Client(t), dir, rundirs))
		require.Equal(t, []string{StatusCompiling, StatusFailed}, cp.Calls())

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Empty(t, qentries)
	})
}

func TestCompileN(t *testing.T) {
	t.Run("removes the compile job directory however the job completes", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		// compiles and is handed off.
		succeeds := uuid.Must(uuid.NewV7())
		encoded, err := json.Marshal(&computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{VcsUri: uri, VcsCommit: ref, Cores: 1},
			AccessToken: "token",
		})
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(succeeds, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(succeeds, eg.EnvironFile, strings.NewReader(fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, uri, eg.EnvGitHeadRef, ref))))
		require.NoError(t, compiledirs.Enqueue(succeeds))

		// fails, missing the head uri/ref.
		fails := uuid.Must(uuid.NewV7())
		encoded, err = json.Marshal(&computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{VcsUri: uri, VcsCommit: ref, Cores: 1},
			AccessToken: "token",
		})
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(fails, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(fails, eg.EnvironFile, strings.NewReader("")))
		require.NoError(t, compiledirs.Enqueue(fails))

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		finished := make(chan error, 1)
		go func() { finished <- CompileN(ctx, 1, vcsaccess, compiledirs, rundirs) }()

		entries := func(dir string) []os.DirEntry {
			dentries, err := os.ReadDir(dir)
			require.NoError(t, err)
			return dentries
		}

		require.Eventually(t, func() bool {
			return len(entries(rundirs.Queued)) == 1 && len(entries(compiledirs.Queued)) == 0 && len(entries(compiledirs.Running)) == 0
		}, time.Minute, 50*time.Millisecond)

		cancel()
		<-finished

		require.Equal(t, Queued().Dirname(succeeds), entries(rundirs.Queued)[0].Name())
		require.Empty(t, entries(compiledirs.Tombstoned))
	})

	t.Run("recovers jobs interrupted mid compile", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		vcsaccess := &http.Client{Transport: roundtripper(func(r *http.Request) (*http.Response, error) {
			w := httptest.NewRecorder()
			require.NoError(t, json.NewEncoder(w).Encode(&compute.GitCredentialsHTTP{Username: "git", Password: "short-lived"}))
			return w.Result(), nil
		})}

		uid := uuid.Must(uuid.NewV7())
		encoded, err := json.Marshal(&computeapi.EnqueuedDequeueResponse{
			Enqueued:    &computeapi.Enqueued{VcsUri: uri, VcsCommit: ref, Cores: 1},
			AccessToken: "token",
		})
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Download(uid, eg.EnvironFile, strings.NewReader(fmt.Sprintf("%s=%s\n%s=%s\n", eg.EnvGitHeadURI, uri, eg.EnvGitHeadRef, ref))))
		require.NoError(t, compiledirs.Enqueue(uid))

		// claimed and partially cloned when the previous process was
		// cancelled, left behind in Running.
		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "partial"), nil, 0600))

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		finished := make(chan error, 1)
		go func() { finished <- CompileN(ctx, 1, vcsaccess, compiledirs, rundirs) }()

		entries := func(dir string) []os.DirEntry {
			dentries, err := os.ReadDir(dir)
			require.NoError(t, err)
			return dentries
		}

		require.Eventually(t, func() bool {
			return len(entries(rundirs.Queued)) == 1 && len(entries(compiledirs.Queued)) == 0 && len(entries(compiledirs.Running)) == 0
		}, 30*time.Second, 50*time.Millisecond)

		cancel()
		<-finished

		require.Equal(t, Queued().Dirname(uid), entries(rundirs.Queued)[0].Name())
	})
}
