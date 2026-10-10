package runners

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/fsx"
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
	t.Run("compiles a local source ref and hands it directly to rundirs.Queued, bypassing Downloading", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		req := EnqueuedDequeueResponse{
			Enqueued: &Enqueued{
				VcsUri:    uri,
				VcsCommit: ref,
				Cores:     1,
			},
		}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.NoError(t, compileWorkload(t.Context(), http.DefaultClient, dir, rundirs))

		// bypasses rundirs.Downloading entirely.
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

		var resp EnqueuedDequeueResponse
		require.NoError(t, json.Unmarshal(mencoded, &resp))
		require.Equal(t, uid.String(), resp.Enqueued.Id)
		require.NotEmpty(t, resp.Enqueued.Entry)
		require.Equal(t, ref, resp.Enqueued.VcsCommit)

		// clone artifacts are cleaned up, not left behind in the run directory.
		require.NoDirExists(t, filepath.Join(target, "src"))

		// the incoming request environment is overwritten by the outgoing
		// workload environment once cloning is done, so any credentials it
		// carried don't linger.
		outgoing, err := os.ReadFile(filepath.Join(target, eg.EnvironFile))
		require.NoError(t, err)
		require.NotContains(t, string(outgoing), "EG_GIT_AUTH_ACCESS_TOKEN")

		// the compile-side job directory is gone -- it was renamed, not copied.
		require.NoDirExists(t, dir)
	})

	t.Run("failed compiles are discarded and never reach rundirs.Queued", func(t *testing.T) {
		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		req := EnqueuedDequeueResponse{Enqueued: &Enqueued{VcsUri: filepath.Join(t.TempDir(), "does-not-exist")}}
		encoded, err := json.Marshal(&req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
		require.NoError(t, compiledirs.Enqueue(uid))

		dir, err := compiledirs.Dequeue()
		require.NoError(t, err)

		require.Error(t, compileWorkload(t.Context(), http.DefaultClient, dir, rundirs))

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
	reserved EnqueuedDequeueResponse
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
	spool := func(t *testing.T, compiledirs SpoolDirs, uid uuid.UUID, req *EnqueuedDequeueResponse) string {
		encoded, err := json.Marshal(req)
		require.NoError(t, err)
		require.NoError(t, compiledirs.Download(uid, "metadata.json", bytes.NewReader(encoded)))
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
			reserved: EnqueuedDequeueResponse{
				Enqueued:    &Enqueued{Id: uid.String(), VcsUri: uri, VcsCommit: ref, Entry: "reserved.wasm"},
				AccessToken: "fresh",
			},
		}

		dir := spool(t, compiledirs, uid, &EnqueuedDequeueResponse{
			Enqueued: &Enqueued{Id: uid.String(), VcsUri: uri, VcsCommit: ref, Cores: 1},
		})

		require.NoError(t, compileWorkloadReserved(t.Context(), http.DefaultClient, cp.Client(t), dir, rundirs))
		require.Equal(t, []string{StatusCompiling, "reserved", StatusQueued}, cp.Calls())

		target := filepath.Join(rundirs.Queued, filepath.Base(dir))
		mencoded, err := os.ReadFile(filepath.Join(target, "metadata.json"))
		require.NoError(t, err)

		// the workload is recorded as the control plane reserved it.
		var resp EnqueuedDequeueResponse
		require.NoError(t, json.Unmarshal(mencoded, &resp))
		require.Equal(t, uid.String(), resp.Enqueued.Id)
		require.Equal(t, "reserved.wasm", resp.Enqueued.Entry)
		require.Equal(t, "fresh", resp.AccessToken)

		// the token travels with the workload so it can report running.
		require.Equal(t, "candidate", ReadCandidateToken(target))
	})

	t.Run("pushed workloads report failures", func(t *testing.T) {
		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		cp := &controlplane{}

		dir := spool(t, compiledirs, uid, &EnqueuedDequeueResponse{
			Enqueued: &Enqueued{Id: uid.String(), VcsUri: filepath.Join(t.TempDir(), "does-not-exist")},
		})

		require.Error(t, compileWorkloadReserved(t.Context(), http.DefaultClient, cp.Client(t), dir, rundirs))
		require.Equal(t, []string{StatusCompiling, StatusFailed}, cp.Calls())

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Empty(t, qentries)
	})

	t.Run("without the control plane pushed workloads fail to reserve", func(t *testing.T) {
		uri, ref := newLocalSourceRepo(t)

		compiledirs := NewSpoolDir(t.TempDir())
		rundirs := NewSpoolDir(t.TempDir())

		uid := uuid.Must(uuid.NewV7())
		dir := spool(t, compiledirs, uid, &EnqueuedDequeueResponse{
			Enqueued: &Enqueued{VcsUri: uri, VcsCommit: ref, Cores: 1},
		})

		require.Error(t, compileWorkload(t.Context(), http.DefaultClient, dir, rundirs))

		qentries, err := os.ReadDir(rundirs.Queued)
		require.NoError(t, err)
		require.Empty(t, qentries)
	})
}
