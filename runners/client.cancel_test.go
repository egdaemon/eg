package runners

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

func TestPushClientCancel(t *testing.T) {
	t.Run("posts the cancellation to the control plane", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/c/q/abc/cancel", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		require.NoError(t, PushClient{c: srv.Client(), host: srv.URL}.Cancel(t.Context(), "abc"))
	})

	t.Run("unknown workload is a not found status error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		err := PushClient{c: srv.Client(), host: srv.URL}.Cancel(t.Context(), "abc")
		require.Error(t, httpx.IsStatusError(err, http.StatusNotFound))
	})
}
