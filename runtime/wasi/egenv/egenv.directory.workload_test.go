package egenv_test

import (
	"os"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

func TestDirectoryWorkload(t *testing.T) {
	t.Run("defaults to the workspace directory", func(t *testing.T) {
		t.Setenv(eg.EnvComputeWorkspaceDirectory, "")
		require.NoError(t, os.Unsetenv(eg.EnvComputeWorkspaceDirectory))
		require.Equal(t, eg.DefaultWorkspaceDirectory(), egenv.DirectoryWorkload())
		require.Equal(t, eg.DefaultWorkspaceDirectory("foo", "bar"), egenv.DirectoryWorkload("foo", "bar"))
	})

	t.Run("environment overrides the directory", func(t *testing.T) {
		t.Setenv(eg.EnvComputeWorkspaceDirectory, "/srv/workspace")
		require.Equal(t, "/srv/workspace", egenv.DirectoryWorkload())
		require.Equal(t, "/srv/workspace/foo/bar", egenv.DirectoryWorkload("foo", "bar"))
	})

	t.Run("deprecated name matches", func(t *testing.T) {
		t.Setenv(eg.EnvComputeWorkspaceDirectory, "/srv/workspace")
		require.Equal(t, egenv.DirectoryWorkload("foo"), egenv.WorkspaceDirectory("foo"))
	})
}
