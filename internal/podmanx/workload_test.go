package podmanx

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestWorkloadEnviron(t *testing.T) {
	conf := func(path string, err error) func() (string, error) {
		return func() (string, error) { return path, err }
	}

	t.Run("a host's podman runs with its own configuration", func(t *testing.T) {
		require.Equal(t, []string{"A=1"}, environ_([]string{"A=1"}, false, conf("/c.conf", nil)))
	})

	t.Run("a workload's podman runs with the workload configuration", func(t *testing.T) {
		require.Equal(t, []string{"A=1", "CONTAINERS_CONF_OVERRIDE=/c.conf"}, environ_([]string{"A=1"}, true, conf("/c.conf", nil)))
	})

	t.Run("the environ given is left alone", func(t *testing.T) {
		environ := make([]string, 1, 2)
		environ[0] = "A=1"
		_ = environ_(environ, true, conf("/c.conf", nil))
		require.Equal(t, []string{"A=1"}, environ)
		require.Equal(t, "", environ[:2][1])
	})

	t.Run("an explicit override wins", func(t *testing.T) {
		require.Equal(t, []string{"CONTAINERS_CONF_OVERRIDE=/x.conf"}, environ_([]string{"CONTAINERS_CONF_OVERRIDE=/x.conf"}, true, conf("/c.conf", nil)))
	})

	t.Run("without the configuration podman runs with its own", func(t *testing.T) {
		require.Equal(t, []string{"A=1"}, environ_([]string{"A=1"}, true, conf("", errors.New("boom"))))
	})

	t.Run("the configuration is written once, named by its content", func(t *testing.T) {
		dir := t.TempDir()
		first, err := writeworkloadconf(dir)
		require.NoError(t, err)
		second, err := writeworkloadconf(dir)
		require.NoError(t, err)
		require.Equal(t, first, second)

		written, err := os.ReadFile(first)
		require.NoError(t, err)
		require.Equal(t, workloadconf, written)
	})

	t.Run("only the podman command sees it, never this process", func(t *testing.T) {
		t.Setenv(eg.EnvComputePodmanWorkload, "true")
		t.Setenv(envconfoverride, "")
		os.Unsetenv(envconfoverride)

		cmd := Command(context.Background(), "version")
		require.True(t, slices.ContainsFunc(cmd.Env, func(kv string) bool {
			return len(kv) > len(envconfoverride) && kv[:len(envconfoverride)+1] == envconfoverride+"="
		}))
		_, set := os.LookupEnv(envconfoverride)
		require.False(t, set)
	})
}
