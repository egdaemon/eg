package execproxy

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConfined runs commands confined with Landlock, which needs a kernel whose Landlock is enabled.
func TestConfined(t *testing.T) {
	// run runs name with args confined and reports its exit code and stderr.
	run := func(t *testing.T, name string, args ...string) (int, string) {
		var (
			stderr bytes.Buffer
		)

		cmd := confined(t.Context(), name, args...)
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			exit, ok := errors.AsType[*exec.ExitError](err)
			require.True(t, ok, err)
			return exit.ExitCode(), stderr.String()
		}
		return 0, stderr.String()
	}

	t.Run("writes beneath the temporary directories", func(t *testing.T) {
		dir, err := os.MkdirTemp("/tmp", "confined")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(dir) })

		code, stderr := run(t, "sh", "-c", "echo written > "+filepath.Join(dir, "file"))
		require.Equal(t, 0, code, stderr)
		b, err := os.ReadFile(filepath.Join(dir, "file"))
		require.NoError(t, err)
		require.Equal(t, "written\n", string(b))
	})

	t.Run("refuses writes anywhere else", func(t *testing.T) {
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		dir, err := os.MkdirTemp(home, "confined")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(dir) })

		code, stderr := run(t, "sh", "-c", "echo written > "+filepath.Join(dir, "file"))
		require.NotEqual(t, 0, code)
		require.Contains(t, stderr, "Permission denied")
		_, err = os.Stat(filepath.Join(dir, "file"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("reads and runs anything", func(t *testing.T) {
		code, stderr := run(t, "cat", "/etc/os-release")
		require.Equal(t, 0, code, stderr)
	})

	t.Run("runs a name the PATH does not hold as it is", func(t *testing.T) {
		err := confined(t.Context(), "eg-does-not-exist").Run()
		require.ErrorIs(t, err, exec.ErrNotFound)
	})
}

func TestPrivileged(t *testing.T) {
	t.Run("sudo running a command as root", func(t *testing.T) {
		require.True(t, privileged("sudo", []string{"-E", "-H", "-u", "root", "-g", "root", "bash", "-c", "true"}))
		require.True(t, privileged("/usr/bin/sudo", []string{"--user=root", "true"}))
		require.True(t, privileged("sudo", []string{"--user", "root", "true"}))
	})

	t.Run("sudo running a command as another user", func(t *testing.T) {
		require.False(t, privileged("sudo", []string{"-E", "-H", "-u", "egd", "-g", "egd", "bash", "-c", "true"}))
	})

	t.Run("sudo naming no user", func(t *testing.T) {
		require.False(t, privileged("sudo", []string{"true"}))
	})

	t.Run("a command other than sudo", func(t *testing.T) {
		require.False(t, privileged("bash", []string{"-u", "root"}))
	})
}
