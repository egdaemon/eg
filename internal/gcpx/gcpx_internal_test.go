package gcpx

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/stretchr/testify/require"
)

func TestWanted(t *testing.T) {
	envfile := func(t *testing.T, content string) string {
		p := filepath.Join(t.TempDir(), eg.EnvironFile)
		require.NoError(t, os.WriteFile(p, []byte(content), 0600))
		return p
	}

	t.Run("should not be wanted by default", func(t *testing.T) {
		require.False(t, wanted(envfile(t, "FOO=bar\n")))
		require.False(t, wanted(filepath.Join(t.TempDir(), "missing")))
	})

	t.Run("should not be wanted when disabled or declaring only the google credentials", func(t *testing.T) {
		require.False(t, wanted(envfile(t, "EG_GCP_WORKLOAD_IDENTITY_ENABLED=false\n")))
		require.False(t, wanted(envfile(t, "GOOGLE_APPLICATION_CREDENTIALS=/anything\n")))
	})

	t.Run("should be wanted when the workload enabled it", func(t *testing.T) {
		require.True(t, wanted(envfile(t, "FOO=bar\nEG_GCP_WORKLOAD_IDENTITY_ENABLED=true\n")))
	})
}

func TestProvision(t *testing.T) {
	// a runtime directory of a workload that enabled gcp workload identity.
	enabled := func(t *testing.T) string {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, eg.EnvironFile), []byte("EG_GCP_WORKLOAD_IDENTITY_ENABLED=true\n"), 0600))
		return dir
	}

	const credentials = `{"id_token":"idtoken","audience":"//iam.googleapis.com/example"}`

	fake := func(response string, err error, calls *int) fetch {
		return func(ctx context.Context, c *http.Client, dst, token string) error {
			*calls++
			require.Equal(t, "token", token)
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dst, responseFilename), []byte(response), 0600)
		}
	}

	t.Run("should not fetch when the workload did not ask", func(t *testing.T) {
		var calls int
		adc, err := provision(t.Context(), fake("", nil, &calls), nil, t.TempDir(), "token")
		require.NoError(t, err)
		require.Empty(t, adc)
		require.Zero(t, calls)
	})

	t.Run("should provision the credentials into the gcloud directory when enabled", func(t *testing.T) {
		var calls int
		dir := enabled(t)
		adc, err := provision(t.Context(), fake(credentials, nil, &calls), nil, dir, "token")
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		require.Equal(t, filepath.Join(dir, Directory, adcFilename), adc)
		require.FileExists(t, adc)
		require.FileExists(t, filepath.Join(dir, Directory, idTokenFilename))
	})

	for _, code := range []int{http.StatusUnauthorized, http.StatusNotFound} {
		t.Run("should run without credentials when the account has no connection", func(t *testing.T) {
			var calls int
			dir := enabled(t)
			_, cause := httpx.AsError(&http.Response{StatusCode: code, Status: http.StatusText(code)}, nil)
			adc, err := provision(t.Context(), fake("", cause, &calls), nil, dir, "token")
			require.NoError(t, err)
			require.Empty(t, adc)
			require.NoDirExists(t, filepath.Join(dir, Directory))
		})
	}

	t.Run("should fail on other errors", func(t *testing.T) {
		var calls int
		_, cause := httpx.AsError(&http.Response{StatusCode: http.StatusInternalServerError, Status: "500"}, nil)
		_, err := provision(t.Context(), fake("", cause, &calls), nil, enabled(t), "token")
		require.Error(t, err)
	})
}
