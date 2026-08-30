package runners

import (
	"testing"

	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestAgentOptionLocalComputeCachingVolumes(t *testing.T) {
	t.Run("github style remote", func(t *testing.T) {
		opt := AgentOptionLocalComputeCachingVolumes("git@github.com:exampleorg/examplerepo.git")
		a := langx.Clone(Agent{}, opt)

		require.Equal(t, []string{
			"--volume", "exampleorg.examplerepo.eg.containers:/var/lib/containers:rw",
		}, a.volumes)
	})

	t.Run("sourcehut style remote with tilde owner", func(t *testing.T) {
		opt := AgentOptionLocalComputeCachingVolumes("git@git.sr.ht:~exampleuser/examplerepo")
		a := langx.Clone(Agent{}, opt)

		require.Equal(t, []string{
			"--volume", "exampleuser.examplerepo.eg.containers:/var/lib/containers:rw",
		}, a.volumes)
	})
}
