package runners

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestAgentOptionSSHServer(t *testing.T) {
	hasVolume := func(a Agent, substr string) bool {
		for _, v := range a.volumes {
			if strings.Contains(v, substr) {
				return true
			}
		}
		return false
	}

	hasLiteral := func(a Agent, substr string) bool {
		for _, l := range a.literals {
			if strings.Contains(l, substr) {
				return true
			}
		}
		return false
	}

	t.Run("port zero disables the feature", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		ws := workspaces.Context{RuntimeDir: t.TempDir()}
		opt := AgentOptionSSHServer(t.Context(), ws, 0)
		a := langx.Clone(Agent{}, opt)

		require.Empty(t, a.volumes)
		require.Empty(t, a.literals)
	})

	t.Run("nonzero port mounts the key and publishes the port", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		ws := workspaces.Context{RuntimeDir: t.TempDir()}
		opt := AgentOptionSSHServer(t.Context(), ws, 2222)
		a := langx.Clone(Agent{}, opt)

		require.True(t, hasVolume(a, "/etc/ssh/authorized_keys/egd"), "expected authorized_keys mount")
		require.True(t, hasVolume(a, "/etc/ssh/sshd_config.d/99-eg.conf"), "expected sshd_config.d drop-in mount")
		require.True(t, hasLiteral(a, "2222:22"), "expected port published to container's sshd")

		authorizedkeys := filepath.Join(ws.RuntimeDir, "ssh", "authorized_keys")
		content, err := os.ReadFile(authorizedkeys)
		require.NoError(t, err)

		_, _, _, _, err = ssh.ParseAuthorizedKey(content)
		require.NoError(t, err, "generated authorized_keys content must parse as a valid public key")

		dropin := filepath.Join(ws.RuntimeDir, "ssh", "sshd_config.d", "99-eg.conf")
		dropincontent, err := os.ReadFile(dropin)
		require.NoError(t, err)
		require.Contains(t, string(dropincontent), "AuthorizedKeysFile /etc/ssh/authorized_keys/%u .ssh/authorized_keys")
	})

	t.Run("reruns reuse the same cached key", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		firstws := workspaces.Context{RuntimeDir: t.TempDir()}
		secondws := workspaces.Context{RuntimeDir: t.TempDir()}
		langx.Clone(Agent{}, AgentOptionSSHServer(t.Context(), firstws, 2222))
		langx.Clone(Agent{}, AgentOptionSSHServer(t.Context(), secondws, 2222))

		firstkey, err := os.ReadFile(filepath.Join(firstws.RuntimeDir, "ssh", "authorized_keys"))
		require.NoError(t, err)
		secondkey, err := os.ReadFile(filepath.Join(secondws.RuntimeDir, "ssh", "authorized_keys"))
		require.NoError(t, err)

		require.Equal(t, string(firstkey), string(secondkey))
	})
}
