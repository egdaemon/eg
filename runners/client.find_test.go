package runners

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/stretchr/testify/require"
)

func TestAwaitLogsToken(t *testing.T) {
	respond := func(w http.ResponseWriter, e *computeapi.Enqueued) {
		b, err := json.Marshal(&computeapi.EnqueuedFindResponse{Enqueued: e})
		require.NoError(t, err)
		_, _ = w.Write(b)
	}

	running := timex.RFC3339Inf().Format(time.RFC3339Nano)

	t.Run("waits for the workload to be scheduled", func(t *testing.T) {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/c/q/abc", r.URL.Path)
			if calls.Add(1) < 3 {
				respond(w, &computeapi.Enqueued{Id: "abc", CompletedAt: running})
				return
			}
			respond(w, &computeapi.Enqueued{Id: "abc", CompletedAt: running, LogsToken: "tok"})
		}))
		defer srv.Close()

		found, err := PushClient{c: srv.Client(), host: srv.URL}.AwaitLogsToken(t.Context(), "abc", time.Millisecond)
		require.NoError(t, err)
		require.Equal(t, "tok", found.LogsToken)
		require.EqualValues(t, 3, calls.Load())
	})

	t.Run("errors when the workload completed without a token", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			respond(w, &computeapi.Enqueued{Id: "abc", CompletedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		}))
		defer srv.Close()

		_, err := PushClient{c: srv.Client(), host: srv.URL}.AwaitLogsToken(t.Context(), "abc", time.Millisecond)
		require.ErrorIs(t, err, ErrWorkloadCompleted)
	})
}

func TestHistoryLogs(t *testing.T) {
	t.Run("streams the stored log", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/c/history/abc/logs", r.URL.Path)
			_, _ = w.Write([]byte("hello\nworld\n"))
		}))
		defer srv.Close()

		var dst bytes.Buffer
		require.NoError(t, PushClient{c: srv.Client(), host: srv.URL}.HistoryLogs(t.Context(), "abc", &dst))
		require.Equal(t, "hello\nworld\n", dst.String())
	})

	t.Run("reports missing logs as not found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		err := PushClient{c: srv.Client(), host: srv.URL}.HistoryLogs(t.Context(), "abc", io.Discard)
		require.Error(t, httpx.IsStatusError(err, http.StatusNotFound))
	})
}

func TestHistoryFind(t *testing.T) {
	t.Run("returns the recorded result", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/c/history/abc", r.URL.Path)
			_, _ = w.Write([]byte(`{"history":{"id":"abc","result":1}}`))
		}))
		defer srv.Close()

		h, err := PushClient{c: srv.Client(), host: srv.URL}.HistoryFind(t.Context(), "abc")
		require.NoError(t, err)
		require.Equal(t, "abc", h.Id)
		require.EqualValues(t, 1, h.Result)
	})

	t.Run("reports an unrecorded workload as not found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		_, err := PushClient{c: srv.Client(), host: srv.URL}.HistoryFind(t.Context(), "abc")
		require.Error(t, httpx.IsStatusError(err, http.StatusNotFound))
	})
}
