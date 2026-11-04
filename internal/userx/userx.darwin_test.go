//go:build darwin

package userx

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultRuntimeDirectory(t *testing.T) {
	u := &user.User{Uid: "501"}

	// sockets are bound in the runtime directory, and darwin limits a unix socket path to 103 bytes.
	const sunpath = 103

	t.Run("the user's temporary directory", func(t *testing.T) {
		t.Setenv(EnvRuntimeDirectory, "")
		t.Setenv("XDG_RUNTIME_DIR", "")
		require.Equal(t, os.TempDir(), defaultRuntimeDirectory(u))
	})

	t.Run("exists and fits the longest socket", func(t *testing.T) {
		t.Setenv(EnvRuntimeDirectory, "")
		t.Setenv("XDG_RUNTIME_DIR", "")
		dir := defaultRuntimeDirectory(u)
		require.DirExists(t, dir)
		require.LessOrEqual(t, len(filepath.Join(dir, "main.01900000-0000-7000-8000-000000000000.socket")), sunpath)
	})

	t.Run("root uses /var/run", func(t *testing.T) {
		t.Setenv(EnvRuntimeDirectory, "")
		t.Setenv("XDG_RUNTIME_DIR", "")
		require.Equal(t, "/var/run", defaultRuntimeDirectory(&user.User{Uid: Root().Uid}))
	})

	t.Run("RUNTIME_DIRECTORY overrides the default", func(t *testing.T) {
		t.Setenv(EnvRuntimeDirectory, "/custom")
		t.Setenv("XDG_RUNTIME_DIR", "/xdg")
		require.Equal(t, "/custom", defaultRuntimeDirectory(u))
	})

	t.Run("XDG_RUNTIME_DIR overrides the default", func(t *testing.T) {
		t.Setenv(EnvRuntimeDirectory, "")
		t.Setenv("XDG_RUNTIME_DIR", "/xdg")
		require.Equal(t, "/xdg", defaultRuntimeDirectory(u))
	})
}
