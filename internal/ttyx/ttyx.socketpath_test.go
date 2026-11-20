package ttyx

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSocketPath(t *testing.T) {
	t.Run("runtime directory", func(t *testing.T) {
		t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
		require.Equal(t, "/run/user/1000/eg/pty.d/abc.sock", SocketPath("abc"))
	})

	t.Run("cache directory", func(t *testing.T) {
		t.Setenv("XDG_RUNTIME_DIR", "")
		t.Setenv("XDG_CACHE_HOME", "/home/u/.cache")
		require.Equal(t, filepath.Join("/home/u/.cache", "eg", "pty.d", "abc.sock"), SocketPath("abc"))
	})
}
