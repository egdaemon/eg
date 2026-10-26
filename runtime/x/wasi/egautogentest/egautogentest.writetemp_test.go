package egautogentest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteTemp(t *testing.T) {
	t.Run("writes the data to a dot file within the directory", func(t *testing.T) {
		dir := t.TempDir()
		path, err := writeTemp(dir, ".autogentest.*", []byte("generated"))
		require.NoError(t, err)
		require.Equal(t, dir, filepath.Dir(path))
		require.True(t, strings.HasPrefix(filepath.Base(path), "."))

		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "generated", string(raw))

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0644), info.Mode().Perm())
	})

	t.Run("creates distinct files", func(t *testing.T) {
		dir := t.TempDir()
		a, err := writeTemp(dir, ".autogentest.*", nil)
		require.NoError(t, err)
		b, err := writeTemp(dir, ".autogentest.*", nil)
		require.NoError(t, err)
		require.NotEqual(t, a, b)
	})

	t.Run("fails when the directory does not exist", func(t *testing.T) {
		_, err := writeTemp(filepath.Join(t.TempDir(), "missing"), ".autogentest.*", nil)
		require.Error(t, err)
	})

	t.Run("can be renamed over an existing file", func(t *testing.T) {
		dir := t.TempDir()
		dest := filepath.Join(dir, "a_test.go")
		require.NoError(t, os.WriteFile(dest, []byte("previous"), 0644))

		path, err := writeTemp(dir, ".autogentest.*", []byte("generated"))
		require.NoError(t, err)
		require.NoError(t, os.Rename(path, dest))

		raw, err := os.ReadFile(dest)
		require.NoError(t, err)
		require.Equal(t, "generated", string(raw))
	})
}
