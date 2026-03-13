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

	t.Run("sets GOOGLE_APPLICATION_CREDENTIALS to workload directory path", func(t *testing.T) {
		tmp := t.TempDir()
		credspath := filepath.Join(tmp, "application_default_credentials.json")
		require.NoError(t, os.WriteFile(credspath, []byte(`{}`), 0600))

		envb := envx.Build()
		AgentOptionGcloudCredentials(t.Context(), envb, credspath)

		environ, err := envb.Environ()
		require.NoError(t, err)
		expected := eg.DefaultWorkloadDirectory("gcloud", "application_default_credentials.json")
		require.Contains(t, environ, EnvGoogleApplicationCredentials+"="+expected)
	})

	t.Run("appends gcloud workload directory to remap env var", func(t *testing.T) {
		tmp := t.TempDir()
		credspath := filepath.Join(tmp, "application_default_credentials.json")
		require.NoError(t, os.WriteFile(credspath, []byte(`{}`), 0600))

		envb := envx.Build()
		AgentOptionGcloudCredentials(t.Context(), envb, credspath)

		environ, err := envb.Environ()
		require.NoError(t, err)
		expected := eg.DefaultWorkloadDirectory("gcloud")
		found := false
		for _, e := range environ {
			if strings.HasPrefix(e, eg.EnvUnsafeRemapDirectory+"=") && strings.Contains(e, expected) {
				found = true
				break
			}
		}
		require.True(t, found, "expected %s to contain %s", eg.EnvUnsafeRemapDirectory, expected)
	})

	t.Run("mounts overlay of credentials directory", func(t *testing.T) {
		tmp := t.TempDir()
		credspath := filepath.Join(tmp, "application_default_credentials.json")
		require.NoError(t, os.WriteFile(credspath, []byte(`{}`), 0600))

		envb := envx.Build()
		opt := AgentOptionGcloudCredentials(t.Context(), envb, credspath)
		a := langx.Clone(Agent{}, opt)

		require.NotEmpty(t, a.volumes)
		require.True(t, hasVolume(a, tmp), "expected volume mount for credentials directory")
		require.True(t, hasVolume(a, eg.DefaultMountRoot("gcloud")), "expected volume mount for gcloud mount root")
	})
}
