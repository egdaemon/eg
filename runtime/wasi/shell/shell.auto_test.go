package shell_test

import (
	"path/filepath"
	"strings"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestAuto(t *testing.T) {
	t.Run("defaults to bash on the host", func(t *testing.T) {
		t.Setenv(_eg.EnvExperimentalGuestShell, "")
		cmd := shell.New("echo hello")
		rec := shell.NewRecorder(&cmd)
		require.NoError(t, shell.Run(t.Context(), cmd))
		require.True(t, strings.HasSuffix(rec.Result(), "bash -c echo hello"), rec.Result())
	})

	// builtins and redirects need no host, so the command completes without bash or an exec proxy.
	t.Run("experiment interprets in the guest", func(t *testing.T) {
		workdir := t.TempDir()
		t.Setenv(_eg.EnvExperimentalGuestShell, "true")
		t.Setenv(_eg.EnvComputeWorkingDirectory, workdir)
		cmd := shell.New("echo hello > out")
		rec := shell.NewRecorder(&cmd)
		require.NoError(t, shell.Run(t.Context(), cmd))
		require.Empty(t, rec.Results())
		require.FileExists(t, filepath.Join(workdir, "out"))
	})
}
