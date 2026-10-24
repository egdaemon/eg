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
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/history"
	"github.com/egdaemon/eg/runners"
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

			err := history.Await(t.Context(), "w1", runners.NewPushClient(chttp).HistoryFind)
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

		require.ErrorIs(t, history.Await(ctx, "w1", runners.NewPushClient(chttp).HistoryFind), context.DeadlineExceeded)
		require.GreaterOrEqual(t, len(seen()), 2)
	})
}

func TestStoredLogs(t *testing.T) {
	t.Run("prints the stored log", func(t *testing.T) {
		chttp, seen := fakeapi(t, map[string]string{"/c/history/w1/logs": "hello\n"})

		require.NoError(t, history.Logs(t.Context(), "w1", os.Stdout, runners.NewPushClient(chttp).HistoryLogs))
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
