package runners

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/computeapi"
	"github.com/stretchr/testify/require"
)

func TestCompletionClientUploadPresentsToken(t *testing.T) {
	t.Run("sends the access token attached to the context as a form field", func(t *testing.T) {
		var fields map[string]string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/c/q/w1/completed", r.URL.Path)
			reader, err := r.MultipartReader()
			require.NoError(t, err)
			fields = map[string]string{}
			for {
				part, err := reader.NextPart()
				if err != nil {
					break
				}
				if part.FileName() == "" {
					value, err := io.ReadAll(part)
					require.NoError(t, err)
					fields[part.FormName()] = string(value)
				}
			}
		}))
		defer srv.Close()

		ctx := computeapi.ContextWithCompletionToken(t.Context(), "signed")
		err := CompletionClient{c: srv.Client(), host: srv.URL}.Upload(ctx, "w1", time.Second, nil, strings.NewReader("logs"), strings.NewReader("analytics"))
		require.NoError(t, err)
		require.Equal(t, "signed", fields["access_token"])
		require.Equal(t, "1000", fields["duration"])
		require.Equal(t, "true", fields["successful"])
	})

	t.Run("omits the field when there is no token", func(t *testing.T) {
		var present bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reader, err := r.MultipartReader()
			require.NoError(t, err)
			for {
				part, err := reader.NextPart()
				if err != nil {
					break
				}
				present = present || part.FormName() == "access_token"
			}
		}))
		defer srv.Close()

		err := CompletionClient{c: srv.Client(), host: srv.URL}.Upload(t.Context(), "w1", time.Second, nil, strings.NewReader("logs"), strings.NewReader("analytics"))
		require.NoError(t, err)
		require.False(t, present)
	})
}

func TestReadAccessToken(t *testing.T) {
	t.Run("returns the token recorded with the workload", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"enqueued":{"id":"w1"},"access_token":"signed"}`), 0600))
		require.Equal(t, "signed", ReadAccessToken(dir))
	})

	t.Run("blank when the workload was handed over without one", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(`{"enqueued":{"id":"w1"}}`), 0600))
		require.Empty(t, ReadAccessToken(dir))
	})

	t.Run("blank when the metadata is missing", func(t *testing.T) {
		require.Empty(t, ReadAccessToken(t.TempDir()))
	})
}
