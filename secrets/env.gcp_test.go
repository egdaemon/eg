package secrets

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestEnvReaderGCP(t *testing.T) {
	t.Run("should inject the gcp credentials into the context of the reads", func(t *testing.T) {
		require.Len(t, gcpcredentials(Env().GCP("/adc.json").context(t.Context())), 1)
		require.Empty(t, gcpcredentials(Env().GCP("").context(t.Context())))
		require.Empty(t, gcpcredentials(t.Context()))
	})

	t.Run("should not touch the process environment", func(t *testing.T) {
		t.Setenv(eg.EnvGoogleApplicationCredentials, "previous")

		_, _ = io.ReadAll(Env().GCP(filepath.Join(t.TempDir(), "missing.json")).NewReader(t.Context(), "gcpsm://project/secret"))
		require.Equal(t, "previous", os.Getenv(eg.EnvGoogleApplicationCredentials))
	})

	t.Run("should use the injected credentials for gcp secrets", func(t *testing.T) {
		// the ambient credentials are not consulted when credentials were injected, an unreadable file is an error.
		_, err := io.ReadAll(Env().GCP(filepath.Join(t.TempDir(), "missing.json")).NewReader(t.Context(), "gcpsm://project/secret"))
		require.Error(t, err)
	})
}
