package runners

import (
	"context"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func TestPodmanWorkload(t *testing.T) {
	declared := "--env=" + eg.EnvComputePodmanWorkload + "=true"
	agentenv := func() []string {
		a := NewRunner(context.Background(), workspaces.Context{}, "id")
		env := []string{}
		for i := 0; i+1 < len(a.environ); i += 2 {
			env = append(env, a.environ[i]+"="+a.environ[i+1])
		}
		return env
	}

	t.Run("a host is not a workload and declares none by default", func(t *testing.T) {
		t.Setenv(eg.EnvComputePodmanWorkload, "")
		t.Setenv(eg.EnvExperimentalPodmanWorkload, "")

		require.False(t, PodmanWorkload())
		require.Empty(t, PodmanWorkloadBuildOptions())
		require.NotContains(t, agentenv(), declared)
	})

	t.Run("a host opted in declares the containers it launches without becoming one", func(t *testing.T) {
		t.Setenv(eg.EnvComputePodmanWorkload, "")
		t.Setenv(eg.EnvExperimentalPodmanWorkload, "true")

		require.False(t, PodmanWorkload())
		require.Empty(t, PodmanWorkloadBuildOptions())
		require.Contains(t, agentenv(), declared)
	})

	t.Run("a declared workload builds within its network and declares the containers it launches", func(t *testing.T) {
		t.Setenv(eg.EnvComputePodmanWorkload, "true")
		t.Setenv(eg.EnvExperimentalPodmanWorkload, "")

		require.True(t, PodmanWorkload())
		require.Equal(t, []string{"--network", "host"}, PodmanWorkloadBuildOptions())
		require.Contains(t, agentenv(), declared)
	})
}
