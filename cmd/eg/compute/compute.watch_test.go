package compute

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/computex"
	"github.com/egdaemon/eg/runners"
	"github.com/go-git/go-git/v6"
	gitconfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/golang-jwt/jwt/v4"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
)

// redirect sends every request, regardless of host, to the test server.
type redirect struct{ to *url.URL }

func (t redirect) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme, r.URL.Host = t.to.Scheme, t.to.Host
	return http.DefaultTransport.RoundTrip(r)
}

// fakeapi serves the endpoints used to follow a workload and records the paths requested.
func fakeapi(t *testing.T, routes map[string]string) (*http.Client, func() []string) {
	var (
		m    sync.Mutex
		seen []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.Lock()
		seen = append(seen, r.URL.Path)
		m.Unlock()

		body, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	return &http.Client{Transport: redirect{to: u}}, func() []string {
		m.Lock()
		defer m.Unlock()
		return append([]string(nil), seen...)
	}
}

func TestWorkloadResult(t *testing.T) {
	for _, c := range []struct {
		name   string
		result string
		err    string
	}{
		{name: "completed", result: `{"history":{"result":2}}`},
		{name: "failure", result: `{"history":{"result":1}}`, err: "workload w1 failed"},
		{name: "cancelled", result: `{"history":{"result":3}}`, err: "workload w1 was cancelled"},
	} {
		t.Run(c.name, func(t *testing.T) {
			chttp, _ := fakeapi(t, map[string]string{"/c/history/w1": c.result})

			err := computex.Await(t.Context(), "w1", runners.NewPushClient(chttp).HistoryFind)
			if c.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, c.err)
			}
		})
	}

	t.Run("unrecorded result retries until the context ends", func(t *testing.T) {
		chttp, seen := fakeapi(t, map[string]string{"/c/history/w1": `{"history":{"result":0}}`})

		ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
		defer cancel()

		require.ErrorIs(t, computex.Await(ctx, "w1", runners.NewPushClient(chttp).HistoryFind), context.DeadlineExceeded)
		require.GreaterOrEqual(t, len(seen()), 2)
	})
}

func TestStoredLogs(t *testing.T) {
	t.Run("prints the stored log", func(t *testing.T) {
		chttp, seen := fakeapi(t, map[string]string{"/c/history/w1/logs": "hello\n"})

		require.NoError(t, computex.Logs(t.Context(), "w1", os.Stdout, runners.NewPushClient(chttp).HistoryLogs))
		require.Equal(t, []string{"/c/history/w1/logs"}, seen())
	})
}

func TestFollowEnqueued(t *testing.T) {
	t.Run("completed workload prints stored logs and reports its result", func(t *testing.T) {
		chttp, seen := fakeapi(t, map[string]string{
			"/c/q/w1":            `{"enqueue":{"id":"w1","completed_at":"2020-01-01T00:00:00Z"}}`,
			"/c/history/w1/logs": "done\n",
			"/c/history/w1":      `{"history":{"result":2}}`,
		})

		require.NoError(t, followenqueued(t.Context(), chttp, "w1"))
		require.Equal(t, []string{"/c/q/w1", "/c/history/w1/logs", "/c/history/w1"}, seen())
	})

	t.Run("completed failed workload returns an error", func(t *testing.T) {
		chttp, _ := fakeapi(t, map[string]string{
			"/c/q/w1":            `{"enqueue":{"id":"w1","completed_at":"2020-01-01T00:00:00Z"}}`,
			"/c/history/w1/logs": "boom\n",
			"/c/history/w1":      `{"history":{"result":1}}`,
		})

		require.ErrorContains(t, followenqueued(t.Context(), chttp, "w1"), "failed")
	})

	t.Run("unknown id", func(t *testing.T) {
		chttp, _ := fakeapi(t, nil)

		require.Error(t, followenqueued(t.Context(), chttp, "missing"))
	})
}

// newlogrunner stands in for a runner's "/egdaemon/proxy" stream handler (cmd/eg/daemons/p2p.go),
// serving the request read off the stream with the given handler.
func newlogrunner(t *testing.T, handler http.Handler) host.Host {
	t.Helper()

	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })

	h.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
		defer s.Close()

		req, err := http.ReadRequest(bufio.NewReader(s))
		if err != nil {
			return
		}

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		_, _ = io.Copy(io.Discard, req.Body)
		_ = rec.Result().Write(s)
	})

	return h
}

func TestFollowEnqueuedLive(t *testing.T) {
	for _, c := range []struct {
		name   string
		result string
		err    string
	}{
		{name: "streams the runner's log then reports success", result: `{"history":{"result":2}}`},
		{name: "streams the runner's log then reports failure", result: `{"history":{"result":1}}`, err: "failed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var offset, auth string

			runner := newlogrunner(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/c/logs/w1", r.URL.Path)
				offset, auth = r.URL.Query().Get("offset"), r.Header.Get("Authorization")
				_, _ = w.Write([]byte("from the runner\n"))
			}))

			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"p2p": runner.ID().String()}).SignedString([]byte("secret"))
			require.NoError(t, err)

			// the runner doubles as the bootstrap peer the client joins the p2p network through.
			bootstrap := fmt.Sprintf("%s/p2p/%s", runner.Addrs()[0], runner.ID())

			chttp, seen := fakeapi(t, map[string]string{
				"/c/runners/meta": fmt.Sprintf(`{"bootstrap":[%q]}`, bootstrap),
				"/c/q/w1":         fmt.Sprintf(`{"enqueue":{"id":"w1","logs_token":%q,"completed_at":"9999-12-31T23:59:59Z"}}`, token),
				"/c/history/w1":   c.result,
			})

			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()

			err = followenqueued(ctx, chttp, "w1")
			if c.err == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, c.err)
			}

			require.Equal(t, "0", offset)
			require.Equal(t, token, auth)
			require.Equal(t, []string{"/c/q/w1", "/c/runners/meta", "/c/history/w1"}, seen())
		})
	}
}

// enqueueapi serves what an upload authenticates against along with the enqueue endpoint. the enqueue endpoint
// rejects what egmeta's QueueService.enqueue rejects: a missing archive, and a vcs uri without a vcs commit
// (see the vcs_commit_required migration). returns the config to point the command at it and the forms it accepted.
func enqueueapi(t *testing.T) (*cmdopts.TLSConfig, func() []url.Values) {
	var (
		m        sync.Mutex
		accepted []url.Values
	)

	json := func(w http.ResponseWriter, v string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(v))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
		json(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
	})
	mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { json(w, `{}`) })
	mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { json(w, `{"profiles":[{"token":"profile"}]}`) })
	mux.HandleFunc("/authn/current", func(w http.ResponseWriter, r *http.Request) { json(w, `{"token":"session"}`) })
	mux.HandleFunc("/c/authz/", func(w http.ResponseWriter, r *http.Request) {
		json(w, fmt.Sprintf(`{"token":{"expires":%d,"bearer":"bearer"}}`, time.Now().Add(time.Hour).UnixMilli()))
	})
	mux.HandleFunc("/c/q/", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if _, _, err := r.FormFile("archive"); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if r.FormValue("vcs_uri") != "" && r.FormValue("vcs_commit") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		m.Lock()
		accepted = append(accepted, r.Form)
		m.Unlock()

		json(w, `{"enqueue":{"id":"w1"}}`)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	return &cmdopts.TLSConfig{
		Wrap: func(c *http.Client) *http.Client {
			c.Transport = redirect{to: u}
			return c
		},
	}, func() []url.Values {
		m.Lock()
		defer m.Unlock()
		return append([]url.Values(nil), accepted...)
	}
}

// uploadrepo creates a git repository with a commit and an origin remote and makes it the working directory.
// also isolates what the commands persist (session, caches, ssh key) within the test.
func uploadrepo(t *testing.T) (dir string, sshkey string) {
	dir = t.TempDir()
	t.Chdir(dir)
	t.Setenv("EG_SESSION_TOKEN", "refresh")
	t.Setenv("CACHE_DIRECTORY", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{"https://example.com/org/repo.git"}})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0600))
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add("README.md")
	require.NoError(t, err)
	_, err = wt.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()}})
	require.NoError(t, err)

	return dir, filepath.Join(t.TempDir(), "id_ed25519")
}

// requirevcs asserts the upload carried what the enqueue endpoint requires of a repository with a remote:
// the commit along with the uri.
func requirevcs(t *testing.T, description string, accepted []url.Values) {
	require.Len(t, accepted, 1)
	require.Equal(t, description, accepted[0].Get("description"))
	require.Equal(t, "git@example.com:org/repo.git", accepted[0].Get("vcs_uri"))
	require.Regexp(t, "^[0-9a-f]{40}$", accepted[0].Get("vcs_commit"))
}
