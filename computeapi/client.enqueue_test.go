package computeapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

func TestPushClientEnqueue(t *testing.T) {
	t.Run("posts the workload to the control plane and returns the recorded workload", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Enqueue struct {
					Description string   `json:"description"`
					VcsURI      string   `json:"vcs_uri"`
					Entry       string   `json:"entrypoint"`
					TTL         string   `json:"ttl"`
					SecretURIs  []string `json:"secret_uris"`
				} `json:"enqueue"`
			}

			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/c/q/workload", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "scheduled workload - nightly", body.Enqueue.Description)
			require.Equal(t, "git@example.com:o/r.git", body.Enqueue.VcsURI)
			require.Equal(t, "release/eg/debian", body.Enqueue.Entry)
			require.Equal(t, "3600000", body.Enqueue.TTL)
			require.Equal(t, []string{"gcpsm://p/s"}, body.Enqueue.SecretURIs)
			_, _ = w.Write([]byte(`{"enqueue":{"id":"w1","entrypoint":"release/eg/debian"}}`))
		}))
		defer srv.Close()

		recorded, err := PushClient{c: srv.Client(), host: srv.URL}.Enqueue(t.Context(), &Enqueued{
			Description: "scheduled workload - nightly",
			VcsUri:      "git@example.com:o/r.git",
			Entry:       "release/eg/debian",
			Ttl:         3600000,
			SecretUris:  []string{"gcpsm://p/s"},
		})
		require.NoError(t, err)
		require.Equal(t, "w1", recorded.GetId())
	})

	t.Run("a rejected workload is a status error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer srv.Close()

		_, err := PushClient{c: srv.Client(), host: srv.URL}.Enqueue(t.Context(), &Enqueued{})
		require.Error(t, httpx.IsStatusError(err, http.StatusBadRequest))
	})

	t.Run("a response without a workload is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		_, err := PushClient{c: srv.Client(), host: srv.URL}.Enqueue(t.Context(), &Enqueued{})
		require.ErrorContains(t, err, "missing workload")
	})
}
