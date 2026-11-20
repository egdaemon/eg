package ffiegcontainer

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestMachine(t *testing.T) {
	t.Run("outside a machine every container runs in a container", func(t *testing.T) {
		t.Setenv(eg.EnvComputeEgiso, "")
		containerfiles.Store("outside", ".eg/Containerfile")

		_, ok := machine("eg")
		require.False(t, ok)
		_, ok = machine("outside")
		require.False(t, ok)
	})

	t.Run("the root container runs in a machine booted from the machine's egiso", func(t *testing.T) {
		t.Setenv(eg.EnvComputeEgiso, "/egiso/root.egiso#sha256:0")

		containerfile, ok := machine("eg")
		require.True(t, ok)
		require.Empty(t, containerfile)
	})

	t.Run("a container that was not built runs in a container", func(t *testing.T) {
		t.Setenv(eg.EnvComputeEgiso, "/egiso/root.egiso#sha256:0")

		_, ok := machine("docker.io/library/ubuntu")
		require.False(t, ok)
	})
}

func TestBuild(t *testing.T) {
	t.Run("in a machine the definition is recorded for the machine to build", func(t *testing.T) {
		t.Setenv(eg.EnvComputeEgiso, "/egiso/root.egiso#sha256:0")

		require.NoError(t, Build(t.Context(), "eg.debuild", "/tmp/Containerfile", nil))
		containerfile, ok := machine("eg.debuild")
		require.True(t, ok)
		require.Equal(t, "/tmp/Containerfile", containerfile)
	})

	t.Run("the root container is not built in a machine", func(t *testing.T) {
		t.Setenv(eg.EnvComputeEgiso, "/egiso/root.egiso#sha256:0")

		require.NoError(t, Build(t.Context(), "eg", "/runtime/eg.default.module", nil))
		containerfile, ok := machine("eg")
		require.True(t, ok)
		require.Empty(t, containerfile)
	})
}
