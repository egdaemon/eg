package runners

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestAgentOptionGcloudCredentials(t *testing.T) {
	hasVolume := func(a Agent, substr string) bool {
		for _, v := range a.volumes {
			if strings.Contains(v, substr) {
				return true
			}
		}
		return false
	}

	t.Run("sets base64 encoded credentials env var", func(t *testing.T) {
		tmp := t.TempDir()
		credspath := filepath.Join(tmp, "application_default_credentials.json")
		content := []byte(`{"type":"authorized_user"}`)
		require.NoError(t, os.WriteFile(credspath, content, 0600))

		envb := envx.Build()
		AgentOptionGcloudCredentials(t.Context(), envb, credspath)

		environ, err := envb.Environ()
		require.NoError(t, err)
		expected := base64.URLEncoding.EncodeToString(content)
		require.Contains(t, environ, eg.EnvUnsafeGcloudADCB64+"="+expected)
	})

	t.Run("does not set GOOGLE_APPLICATION_CREDENTIALS", func(t *testing.T) {
		tmp := t.TempDir()
		credspath := filepath.Join(tmp, "application_default_credentials.json")
		require.NoError(t, os.WriteFile(credspath, []byte(`{}`), 0600))

		envb := envx.Build()
		AgentOptionGcloudCredentials(t.Context(), envb, credspath)

		environ, err := envb.Environ()
		require.NoError(t, err)
		for _, e := range environ {
			require.False(t, strings.HasPrefix(e, eg.EnvGoogleApplicationCredentials+"="), "the runner installs the credentials, got %s", e)
		}
	})

	t.Run("does not remap directories", func(t *testing.T) {
		tmp := t.TempDir()
		credspath := filepath.Join(tmp, "application_default_credentials.json")
		require.NoError(t, os.WriteFile(credspath, []byte(`{}`), 0600))

		envb := envx.Build()
		AgentOptionGcloudCredentials(t.Context(), envb, credspath)

		environ, err := envb.Environ()
		require.NoError(t, err)
		for _, e := range environ {
			require.False(t, strings.HasPrefix(e, eg.EnvUnsafeRemapDirectory+"="), "the runner installs the credentials, got %s", e)
		}
	})

	t.Run("does not mount the credentials directory", func(t *testing.T) {
		tmp := t.TempDir()
		credspath := filepath.Join(tmp, "application_default_credentials.json")
		require.NoError(t, os.WriteFile(credspath, []byte(`{}`), 0600))

		envb := envx.Build()
		opt := AgentOptionGcloudCredentials(t.Context(), envb, credspath)
		a := langx.Clone(Agent{}, opt)

		require.False(t, hasVolume(a, tmp), "expected no volume mount for credentials directory")
		require.Empty(t, a.volumes)
	})
}
