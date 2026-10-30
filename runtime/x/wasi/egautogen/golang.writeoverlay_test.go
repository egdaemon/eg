package egautogen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteOverlay(t *testing.T) {
	t.Run("replaces the destination with the source using absolute paths", func(t *testing.T) {
		dir := t.TempDir()
		path, err := writeOverlay(dir, filepath.Join(dir, "a_test.go"), filepath.Join(dir, ".autogen.1"))
		require.NoError(t, err)

		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		require.JSONEq(t, `{"Replace": {"`+filepath.Join(dir, "a_test.go")+`": "`+filepath.Join(dir, ".autogen.1")+`"}}`, string(raw))
	})

	t.Run("is a dot file within the directory", func(t *testing.T) {
		dir := t.TempDir()
		path, err := writeOverlay(dir, "a_test.go", ".autogen.1")
		require.NoError(t, err)
		require.Equal(t, dir, filepath.Dir(path))
		require.True(t, strings.HasPrefix(filepath.Base(path), "."))
	})
}
