package userx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultDirLocation(t *testing.T) {
	t.Run("an unset XDG_CONFIG_HOME is not the working directory", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Chdir(t.TempDir())
		require.NoError(t, os.Mkdir(DefaultDir, 0700))
		require.True(t, filepath.IsAbs(DefaultConfigLocation("wasi.mod.d")))
		require.True(t, filepath.IsAbs(DefaultUserConfigLocation("config.toml")))
	})

	t.Run("prefers XDG_CONFIG_HOME when it exists", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", dir)
		require.NoError(t, os.Mkdir(filepath.Join(dir, DefaultDir), 0700))
		require.Equal(t, filepath.Join(dir, DefaultDir, "wasi.mod.d"), DefaultConfigLocation("wasi.mod.d"))
	})
}
