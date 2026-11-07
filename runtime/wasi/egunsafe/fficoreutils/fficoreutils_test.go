package fficoreutils_test

import (
	"os"
	"path/filepath"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoreutils"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCommand(t *testing.T) {
	t.Run("args include the command name", func(t *testing.T) {
		cmd := fficoreutils.Command(t.Context(), "ls", "-la", "/work")
		require.Equal(t, "ls", cmd.Path)
		require.Equal(t, []string{"ls", "-la", "/work"}, cmd.Args)
	})

	t.Run("without the reactor the utility is not found and the process is removed", func(t *testing.T) {
		runtimedir := t.TempDir()
		t.Setenv(_eg.EnvComputeRuntimeDirectory, runtimedir)

		err := fficoreutils.Command(t.Context(), "ls").Run()
		require.Equal(t, codes.NotFound, status.Code(err))

		procs, err := os.ReadDir(filepath.Join(runtimedir, "proc"))
		require.NoError(t, err)
		require.Empty(t, procs)
	})
}
