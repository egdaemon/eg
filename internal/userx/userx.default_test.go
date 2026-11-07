//go:build !darwin

package userx

import (
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultRuntimeDirectory(t *testing.T) {
	u := &user.User{Uid: "1000", HomeDir: "/home/someone"}

	t.Run("a per user directory within /run/user", func(t *testing.T) {
		t.Setenv(EnvRuntimeDirectory, "")
		t.Setenv("XDG_RUNTIME_DIR", "")
		require.Equal(t, "/run/user/1000", defaultRuntimeDirectory(u))
	})

	t.Run("root uses /run", func(t *testing.T) {
		t.Setenv(EnvRuntimeDirectory, "")
		t.Setenv("XDG_RUNTIME_DIR", "")
		require.Equal(t, "/run", defaultRuntimeDirectory(&user.User{Uid: Root().Uid}))
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

	t.Run("root honors overrides", func(t *testing.T) {
		t.Setenv(EnvRuntimeDirectory, "/custom")
		t.Setenv("XDG_RUNTIME_DIR", "")
		require.Equal(t, "/custom", defaultRuntimeDirectory(&user.User{Uid: Root().Uid}))
	})
}
