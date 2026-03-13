package runners

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestAgentOptionLocalGPGAgent(t *testing.T) {
	hasVolume := func(a Agent, substr string) bool {
		for _, v := range a.volumes {
			if strings.Contains(v, substr) {
				return true
			}
		}
		return false
	}

	t.Run("missing agent socket returns noop", func(t *testing.T) {
		tmp := t.TempDir()
		gnupghome := filepath.Join(tmp, "gnupg")
		require.NoError(t, os.Mkdir(gnupghome, 0700))
		agentsock := filepath.Join(tmp, "S.gpg-agent") // does not exist

		envb := envx.Build()
		opt := agentOptionLocalGPGAgent(t.Context(), envb, agentsock, gnupghome)
		a := langx.Clone(Agent{}, opt)
		require.Empty(t, a.volumes)
	})

	t.Run("missing gnupghome returns noop", func(t *testing.T) {
		tmp := t.TempDir()
		agentsock := filepath.Join(tmp, "S.gpg-agent")
		require.NoError(t, os.WriteFile(agentsock, nil, 0600))
		gnupghome := filepath.Join(tmp, "gnupg") // does not exist

		envb := envx.Build()
		opt := agentOptionLocalGPGAgent(t.Context(), envb, agentsock, gnupghome)
		a := langx.Clone(Agent{}, opt)
		require.Empty(t, a.volumes)
	})

	t.Run("both present mounts volumes and sets GNUPGHOME", func(t *testing.T) {
		tmp := t.TempDir()
		agentsock := filepath.Join(tmp, "S.gpg-agent")
		require.NoError(t, os.WriteFile(agentsock, nil, 0600))
		gnupghome := filepath.Join(tmp, "gnupg")
		require.NoError(t, os.Mkdir(gnupghome, 0700))

		envb := envx.Build()
		opt := agentOptionLocalGPGAgent(t.Context(), envb, agentsock, gnupghome)
		a := langx.Clone(Agent{}, opt)

		require.NotEmpty(t, a.volumes)
		require.True(t, hasVolume(a, gnupghome), "expected gnupghome mount")
		require.True(t, hasVolume(a, agentsock), "expected agent socket mount")

		environ, err := envb.Environ()
		require.NoError(t, err)
		require.Contains(t, environ, "GNUPGHOME="+eg.DefaultMountRoot(".gnupg"))
	})
}
