package runners

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirectClientReserve(t *testing.T) {
	t.Run("uploads the archive with the candidate token", func(t *testing.T) {
		var (
			token, vram, archive string
			labels               []string
		)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/c/q/reserved", r.URL.Path)
			token = r.FormValue("token")
			vram = r.FormValue("vram")
			labels = r.MultipartForm.Value["labels"]

			f, _, err := r.FormFile("archive")
			require.NoError(t, err)
			defer f.Close()
			b, err := io.ReadAll(f)
			require.NoError(t, err)
			archive = string(b)

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"enqueued":{"id":"recorded-id","account_id":"acct"},"access_token":"tok"}`))
		}))
		defer srv.Close()

		c := DirectClient{c: srv.Client(), host: srv.URL}
		recorded, err := c.Reserve(t.Context(), &Enqueued{Id: "pushed-id", Vram: 8, Labels: []string{"gpu", "fast"}}, strings.NewReader("kernel contents"), "candidate-token")
		require.NoError(t, err)
		require.Equal(t, "candidate-token", token)
		require.Equal(t, "8", vram)
		require.Equal(t, []string{"gpu", "fast"}, labels)
		require.Equal(t, "kernel contents", archive)
		require.Equal(t, "recorded-id", recorded.Enqueued.Id)
		require.Equal(t, "tok", recorded.AccessToken)
	})

	t.Run("rejected reservations are errors", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		c := DirectClient{c: srv.Client(), host: srv.URL}
		_, err := c.Reserve(t.Context(), &Enqueued{Id: "pushed-id"}, strings.NewReader("kernel contents"), "candidate-token")
		require.Error(t, err)
	})
}
