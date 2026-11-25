package runners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestAgentOptionSSHKnownHosts(t *testing.T) {
	t.Run("missing file leaves the container's known hosts in place", func(t *testing.T) {
		a := langx.Clone(Agent{}, AgentOptionSSHKnownHosts(filepath.Join(t.TempDir(), "known_hosts")))
		require.Empty(t, a.volumes)
	})

	t.Run("directory is not mounted", func(t *testing.T) {
		a := langx.Clone(Agent{}, AgentOptionSSHKnownHosts(t.TempDir()))
		require.Empty(t, a.volumes)
	})

	t.Run("existing file is mounted read only", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "known_hosts")
		require.NoError(t, os.WriteFile(path, []byte("github.com ssh-ed25519 AAAA\n"), 0600))

		a := langx.Clone(Agent{}, AgentOptionSSHKnownHosts(path))
		require.Equal(t, []string{"--volume", path + ":/etc/ssh/ssh_known_hosts:ro"}, a.volumes)
	})
}
