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

func TestModuleEnv(t *testing.T) {
	// the environment derived from the repository of the workspace, a nested workload is cloned from the path of the
	// workload that launched it.
	local, err := envx.Build().
		Var(eg.EnvComputeVCS, "git@:workload/eg").
		Var(eg.EnvGitHeadVCS, "git@:workload/eg").
		Var(eg.EnvGitBaseVCS, "git@:workload/eg").
		Var(eg.EnvGitHeadCommit, "1111111111111111111111111111111111111111").
		Var("A", "1").
		Environ()
	require.NoError(t, err)

	// the environment of the workload that launched this one.
	parent, err := envx.Build().
		Var("HOME", "/home/example").
		Var(eg.EnvComputeVCS, "git@github.com:egdaemon/eg.git").
		Var(eg.EnvGitHeadVCS, "git@github.com:egdaemon/eg.git").
		Var(eg.EnvGitBaseVCS, "git@github.com:egdaemon/eg.git").
		Var(eg.EnvGitHeadCommit, "2222222222222222222222222222222222222222").
		Var(eg.EnvComputeWorkspaceDirectory, "/w/.eg.workload/m.aaaaaaaa.d/.eg.workspace").
		Environ()
	require.NoError(t, err)

	final := func(environ []string) map[string]string {
		// the last entry of a key wins, as it does for the wasm module configuration.
		m := map[string]string{}
		for _, e := range environ {
			k, v, _ := strings.Cut(e, "=")
			m[k] = v
		}
		return m
	}

	t.Run("the vcs of the parent wins over the one derived from the workspace", func(t *testing.T) {
		environ, err := moduleenv(envx.Build().FromEnviron(local...), parent...).Environ()
		require.NoError(t, err)

		m := final(environ)
		require.Equal(t, "git@github.com:egdaemon/eg.git", m[eg.EnvComputeVCS])
		require.Equal(t, "git@github.com:egdaemon/eg.git", m[eg.EnvGitHeadVCS])
		require.Equal(t, "git@github.com:egdaemon/eg.git", m[eg.EnvGitBaseVCS])
	})

	t.Run("nothing else of the parent survives", func(t *testing.T) {
		environ, err := moduleenv(envx.Build().FromEnviron(local...), parent...).Environ()
		require.NoError(t, err)

		m := final(environ)
		require.Equal(t, "1111111111111111111111111111111111111111", m[eg.EnvGitHeadCommit])
		require.Equal(t, "1", m["A"])
		require.NotContains(t, m, "HOME")
	})

	t.Run("the directories are those of the container", func(t *testing.T) {
		environ, err := moduleenv(envx.Build().FromEnviron(local...), parent...).Environ()
		require.NoError(t, err)

		m := final(environ)
		require.Equal(t, eg.DefaultRootDirectory(), m[eg.EnvComputeWorkloadDirectory])
		require.Equal(t, eg.DefaultWorkingDirectory(), m[eg.EnvComputeWorkingDirectory])
		require.Equal(t, eg.DefaultCacheDirectory(), m[eg.EnvComputeCacheDirectory])
		require.Equal(t, eg.DefaultRuntimeDirectory(), m[eg.EnvComputeRuntimeDirectory])
		require.Equal(t, eg.DefaultWorkspaceDirectory(), m[eg.EnvComputeWorkspaceDirectory])
	})

	t.Run("without an inherited environment the vcs of the workspace is kept", func(t *testing.T) {
		environ, err := moduleenv(envx.Build().FromEnviron(local...)).Environ()
		require.NoError(t, err)

		m := final(environ)
		require.Equal(t, "git@:workload/eg", m[eg.EnvComputeVCS])
		require.Equal(t, "git@:workload/eg", m[eg.EnvGitHeadVCS])
		require.Equal(t, "git@:workload/eg", m[eg.EnvGitBaseVCS])
	})

	t.Run("a blank inherited vcs does not replace the one of the workspace", func(t *testing.T) {
		environ, err := moduleenv(envx.Build().FromEnviron(local...), eg.EnvComputeVCS+"=", eg.EnvGitHeadVCS).Environ()
		require.NoError(t, err)

		m := final(environ)
		require.Equal(t, "git@:workload/eg", m[eg.EnvComputeVCS])
		require.Equal(t, "git@:workload/eg", m[eg.EnvGitHeadVCS])
	})
}
