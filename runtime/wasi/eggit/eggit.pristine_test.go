package eggit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestPristine(t *testing.T) {
	t.Run("clean repository", func(t *testing.T) {
		dir := t.TempDir()
		gitInit(t, dir)
		t.Chdir(dir)

		require.NoError(t, pristine(shell.NewLocal())(t.Context(), nil))
	})

	t.Run("modified tracked file", func(t *testing.T) {
		dir := t.TempDir()
		gitInit(t, dir)
		gitCommitFiles(t, dir, []string{"file.txt"}, "add file")
		t.Chdir(dir)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed"), 0600))

		require.Error(t, pristine(shell.NewLocal())(t.Context(), nil))
	})
}
