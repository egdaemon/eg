package workspaces_test

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func TestNewQueuedRoot(t *testing.T) {
	t.Run("roots in the directory", func(t *testing.T) {
		cwd := t.TempDir()
		t.Setenv(eg.EnvComputeWorkloadDirectory, "")

		ws, err := workspaces.NewQueued(t.Context(), sha256.New(), cwd, "")
		require.NoError(t, err)
		require.Equal(t, cwd, ws.Root)
		require.Equal(t, filepath.Join(cwd, eg.RuntimeDirectory), ws.RuntimeDir)
		require.Equal(t, filepath.Join(cwd, eg.WorkingDirectory), ws.WorkingDir)
		require.DirExists(t, ws.RuntimeDir)
	})

	t.Run("does not root beside the workload that launched it", func(t *testing.T) {
		shared := filepath.Join(t.TempDir(), eg.WorkloadDirectory)
		cwd := filepath.Join(shared, "m.aaaaaaaa.d", eg.WorkingDirectory)
		require.NoError(t, os.MkdirAll(cwd, 0755))
		t.Setenv(eg.EnvComputeWorkloadDirectory, filepath.Join(shared, "m.aaaaaaaa.d"))

		ws, err := workspaces.NewQueued(t.Context(), sha256.New(), cwd, "")
		require.NoError(t, err)
		require.Equal(t, cwd, ws.Root)
	})
}
