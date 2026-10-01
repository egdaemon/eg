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
		keypath := filepath.Join(t.TempDir(), ".ssh", "eg")

		ws := workspaces.Context{RuntimeDir: t.TempDir()}
		opt := AgentOptionSSHServer(t.Context(), ws, keypath, 0)
		a := langx.Clone(Agent{}, opt)

		require.Empty(t, a.volumes)
		require.Empty(t, a.literals)
	})

	t.Run("nonzero port mounts the key and publishes the port", func(t *testing.T) {
		keypath := filepath.Join(t.TempDir(), ".ssh", "eg")

		ws := workspaces.Context{RuntimeDir: t.TempDir()}
		opt := AgentOptionSSHServer(t.Context(), ws, keypath, 2222)
		a := langx.Clone(Agent{}, opt)

		require.True(t, hasVolume(a, "/etc/ssh/authorized_keys/egd"), "expected authorized_keys mount")
		require.True(t, hasVolume(a, "/etc/ssh/sshd_config.d/99-eg.conf"), "expected sshd_config.d drop-in mount")
		require.True(t, hasVolume(a, "/etc/profile.d/99-eg-ssh.sh"), "expected profile.d mount")
		require.True(t, hasLiteral(a, "2222:22"), "expected port published to container's sshd")

		authorizedkeys := filepath.Join(ws.RuntimeDir, "ssh", "etc", "ssh", "authorized_keys", "egd")
		content, err := os.ReadFile(authorizedkeys)
		require.NoError(t, err)

		_, _, _, _, err = ssh.ParseAuthorizedKey(content)
		require.NoError(t, err, "generated authorized_keys content must parse as a valid public key")

		profile, err := os.ReadFile(filepath.Join(ws.RuntimeDir, "ssh", "etc", "profile.d", "99-eg-ssh.sh"))
		require.NoError(t, err)
		require.Contains(t, string(profile), "cd /workload/eg")

		dropin := filepath.Join(ws.RuntimeDir, "ssh", "etc", "ssh", "sshd_config.d", "99-eg.conf")
		dropincontent, err := os.ReadFile(dropin)
		require.NoError(t, err)
		require.Contains(t, string(dropincontent), "AuthorizedKeysFile /etc/ssh/authorized_keys/%u .ssh/authorized_keys")
	})

	t.Run("reruns reuse the same cached key", func(t *testing.T) {
		keypath := filepath.Join(t.TempDir(), ".ssh", "eg")

		firstws := workspaces.Context{RuntimeDir: t.TempDir()}
		secondws := workspaces.Context{RuntimeDir: t.TempDir()}
		langx.Clone(Agent{}, AgentOptionSSHServer(t.Context(), firstws, keypath, 2222))
		langx.Clone(Agent{}, AgentOptionSSHServer(t.Context(), secondws, keypath, 2222))

		firstkey, err := os.ReadFile(filepath.Join(firstws.RuntimeDir, "ssh", "etc", "ssh", "authorized_keys", "egd"))
		require.NoError(t, err)
		secondkey, err := os.ReadFile(filepath.Join(secondws.RuntimeDir, "ssh", "etc", "ssh", "authorized_keys", "egd"))
		require.NoError(t, err)

		require.Equal(t, string(firstkey), string(secondkey))
	})

	t.Run("repeated runs against the same runtime directory succeed", func(t *testing.T) {
		keypath := filepath.Join(t.TempDir(), ".ssh", "eg")

		ws := workspaces.Context{RuntimeDir: t.TempDir()}
		a := langx.Clone(Agent{}, AgentOptionSSHServer(t.Context(), ws, keypath, 2222))
		require.True(t, hasVolume(a, "/etc/ssh/authorized_keys/egd"))

		a = langx.Clone(Agent{}, AgentOptionSSHServer(t.Context(), ws, keypath, 2222))
		require.True(t, hasVolume(a, "/etc/ssh/authorized_keys/egd"), "second run must not disable ssh access")
	})
}
