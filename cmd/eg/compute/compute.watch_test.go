package compute

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

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

			err := workloadresult(t.Context(), chttp, "w1")
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

		require.ErrorIs(t, workloadresult(ctx, chttp, "w1"), context.DeadlineExceeded)
		require.GreaterOrEqual(t, len(seen()), 2)
	})
}

func TestStoredLogs(t *testing.T) {
	t.Run("prints the stored log", func(t *testing.T) {
		chttp, seen := fakeapi(t, map[string]string{"/c/history/w1/logs": "hello\n"})

		require.NoError(t, storedlogs(t.Context(), chttp, "w1"))
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
