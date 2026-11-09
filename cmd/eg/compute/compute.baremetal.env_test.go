package compute

import (
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func TestBaremetalEnv(t *testing.T) {
	ws := workspaces.Context{
		Root:         "/w/.eg.workload/m.bbbbbbbb.d",
		RuntimeDir:   "/w/.eg.workload/m.bbbbbbbb.d/.eg.runtime",
		CacheDir:     "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache",
		WorkspaceDir: "/w/.eg.workload/m.bbbbbbbb.d/.eg.workspace",
		WorkingDir:   "/w/.eg.workload/m.bbbbbbbb.d/eg",
	}

	// the environment of a workload nested in another, the directories are those of the workload that launched it.
	parent := []string{
		"HOME=/home/example",
		eg.EnvComputeWorkloadDirectory + "=/w/.eg.workload/m.aaaaaaaa.d",
		eg.EnvComputeRuntimeDirectory + "=/w/.eg.workload/m.aaaaaaaa.d/.eg.runtime",
		eg.EnvComputeCacheDirectory + "=/w/.eg.workload/m.aaaaaaaa.d/.eg.cache",
		eg.EnvComputeWorkspaceDirectory + "=/w/.eg.workload/m.aaaaaaaa.d/.eg.workspace",
		eg.EnvComputeWorkingDirectory + "=/w/.eg.workload/m.aaaaaaaa.d/eg",
	}

	t.Run("the directories of the workspace win over those of the parent", func(t *testing.T) {
		environ, err := baremetalenv(envx.Build(), ws, parent...).Environ()
		require.NoError(t, err)

		// the last entry of a key wins, as it does for both exec and the wasm module configuration.
		final := map[string]string{}
		for _, e := range environ {
			k, v, _ := strings.Cut(e, "=")
			final[k] = v
		}

		require.Equal(t, ws.Root, final[eg.EnvComputeWorkloadDirectory])
		require.Equal(t, ws.RuntimeDir, final[eg.EnvComputeRuntimeDirectory])
		require.Equal(t, ws.CacheDir, final[eg.EnvComputeCacheDirectory])
		require.Equal(t, ws.WorkspaceDir, final[eg.EnvComputeWorkspaceDirectory])
		require.Equal(t, ws.WorkingDir, final[eg.EnvComputeWorkingDirectory])
	})

	t.Run("the rest of the environment of the parent is kept", func(t *testing.T) {
		environ, err := baremetalenv(envx.Build(), ws, parent...).Environ()
		require.NoError(t, err)
		require.Contains(t, environ, "HOME=/home/example")
	})

	t.Run("without an inherited environment only the directories of the workspace are provided", func(t *testing.T) {
		environ, err := baremetalenv(envx.Build(), ws).Environ()
		require.NoError(t, err)
		require.Contains(t, environ, eg.EnvComputeRuntimeDirectory+"="+ws.RuntimeDir)
		require.NotContains(t, environ, "HOME=/home/example")
		for _, e := range environ {
			require.NotContains(t, e, "m.aaaaaaaa.d")
		}
	})
}
