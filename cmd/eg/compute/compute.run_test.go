package compute

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/schedules"
	"github.com/stretchr/testify/require"
)

func TestRunEnqueue(t *testing.T) {
	t.Run("enqueues the schedule of the workload even when it is paused", func(t *testing.T) {
		var enqueued string
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				require.Equal(t, "/r/schedule/", r.URL.Path)
				require.Equal(t, "https://example.com/r.git", r.URL.Query().Get("vcs_uri"))
				require.Equal(t, "100", r.URL.Query().Get("limit"))
				_, _ = w.Write([]byte(`{"next":{},"items":[{"id":"s1","entrypoint":"other"},{"id":"s2","entrypoint":"release/eg/debian","description":"debian","vcs_uri":"https://example.com/r.git","paused_at":"2020-01-01T00:00:00Z","cores":"4","ttl":"3600000000000"}]}`))
				return
			}

			require.Equal(t, "/c/q/workload", r.URL.Path)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			enqueued = string(body)
			_, _ = w.Write([]byte(`{"enqueued":{"id":"w1"}}`))
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		require.NoError(t, run{VCS: cmdopts.VCS{VcsURI: "https://example.com/r.git"}, Name: "release/eg/debian"}.enqueue(t.Context(), srv.Client()))
		require.Contains(t, enqueued, `"description":"scheduled workload - debian"`)
		require.Contains(t, enqueued, `"entrypoint":"release/eg/debian"`)
		require.Contains(t, enqueued, `"ttl":"3600000"`)
		require.Contains(t, enqueued, `"cores":"4"`)
	})

	t.Run("an ambiguous workload enqueues nothing and lists the candidates", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method, "nothing may be enqueued")
			_, _ = w.Write([]byte(`{"next":{},"items":[{"id":"s1","entrypoint":"w","cron":"7d"},{"id":"s2","entrypoint":"w","cron":"1d"}]}`))
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		err := run{Name: "w"}.enqueue(t.Context(), srv.Client())
		require.ErrorContains(t, err, "ambiguous")
		require.ErrorContains(t, err, "--id")
		require.ErrorContains(t, err, "id s1 workload w schedule 7d")
		require.ErrorContains(t, err, "id s2 workload w schedule 1d")
	})

	t.Run("id selects between ambiguous schedules", func(t *testing.T) {
		var enqueued string
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"next":{},"items":[{"id":"s1","entrypoint":"w","description":"first"},{"id":"s2","entrypoint":"w","description":"second"}]}`))
				return
			}

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			enqueued = string(body)
			_, _ = w.Write([]byte(`{"enqueued":{"id":"w1"}}`))
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		require.NoError(t, run{Name: "w", ID: "s2"}.enqueue(t.Context(), srv.Client()))
		require.Contains(t, enqueued, `"description":"scheduled workload - second"`)
	})

	t.Run("an unknown workload enqueues nothing", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method, "nothing may be enqueued")
			_, _ = w.Write([]byte(`{"next":{},"items":[{"id":"s1","entrypoint":"other"}]}`))
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		err := run{VCS: cmdopts.VCS{VcsURI: "https://example.com/r.git"}, Name: "missing"}.enqueue(t.Context(), srv.Client())
		require.ErrorContains(t, err, "no scheduled workload matched in https://example.com/r.git")
	})
}

func TestMatchSchedule(t *testing.T) {
	items := []*schedules.Schedule{
		{Id: "s1", Entry: "release/eg/debian", Cron: "7d"},
		{Id: "s2", Entry: "release/egworkload/debian", Cron: "7d"},
		{Id: "s3", Entry: "release/eg/debian", Cron: "1d", PausedAt: "2020-01-01T00:00:00Z"},
	}

	t.Run("matches the workload exactly", func(t *testing.T) {
		s, err := matchSchedule(items[:2], "", "release/egworkload/debian", "")
		require.NoError(t, err)
		require.Equal(t, "s2", s.GetId())
	})

	t.Run("a prefix of a workload does not match", func(t *testing.T) {
		_, err := matchSchedule(items[:2], "", "release/eg", "")
		require.ErrorContains(t, err, "no scheduled workload matched")
	})

	t.Run("ambiguous matches report whether each candidate is paused", func(t *testing.T) {
		_, err := matchSchedule(items, "", "release/eg/debian", "")
		require.ErrorContains(t, err, "ambiguous, 2 scheduled workloads matched")
		require.ErrorContains(t, err, "id s1 workload release/eg/debian schedule 7d active")
		require.ErrorContains(t, err, "id s3 workload release/eg/debian schedule 1d paused")
	})

	t.Run("id resolves an ambiguous workload", func(t *testing.T) {
		s, err := matchSchedule(items, "", "release/eg/debian", "s3")
		require.NoError(t, err)
		require.Equal(t, "s3", s.GetId())
	})

	t.Run("id alone selects a schedule", func(t *testing.T) {
		s, err := matchSchedule(items, "", "", "s2")
		require.NoError(t, err)
		require.Equal(t, "release/egworkload/debian", s.GetEntry())
	})

	t.Run("an id of another workload does not match", func(t *testing.T) {
		_, err := matchSchedule(items, "", "release/eg/debian", "s2")
		require.ErrorContains(t, err, "no scheduled workload matched")
	})
}

func TestAwaitWorkload(t *testing.T) {
	t.Run("waits through not found until the workload is queued", func(t *testing.T) {
		var finds int
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/c/history/w1" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			require.Equal(t, "/c/q/w1", r.URL.Path)
			finds++
			if finds < 3 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`{"enqueued":{"id":"w1"}}`))
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		queued, err := awaitworkload(t.Context(), srv.Client(), "w1", time.Millisecond, time.Minute)
		require.NoError(t, err)
		require.True(t, queued)
		require.Equal(t, 3, finds)
	})

	t.Run("a workload that already completed is only in the history", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/c/q/w1" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			require.Equal(t, "/c/history/w1", r.URL.Path)
			_, _ = w.Write([]byte(`{"history":{"id":"w1","result":1}}`))
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		queued, err := awaitworkload(t.Context(), srv.Client(), "w1", time.Millisecond, time.Minute)
		require.NoError(t, err)
		require.False(t, queued)
	})

	t.Run("gives up when the workload never appears", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		_, err := awaitworkload(t.Context(), srv.Client(), "w1", time.Millisecond, 50*time.Millisecond)
		require.ErrorContains(t, err, "was never queued")
	})

	t.Run("a failure other than not found is returned", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		_, err := awaitworkload(t.Context(), srv.Client(), "w1", time.Millisecond, time.Minute)
		require.ErrorContains(t, err, "unable to locate the workload on the cluster")
	})
}
