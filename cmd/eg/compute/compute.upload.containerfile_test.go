package compute

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestUploadContainerfile(t *testing.T) {
	dir := t.TempDir()
	moduledir := eg.DefaultModuleDirectory(dir)
	require.NoError(t, os.MkdirAll(filepath.Join(moduledir, "workload"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(moduledir, "Containerfile"), []byte("FROM scratch\n"), 0600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "custom"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "custom", "Containerfile"), []byte("FROM scratch\n"), 0600))

	t.Run("by default the nearest one is shipped", func(t *testing.T) {
		path, err := upload{Dir: dir, Name: "workload"}.containerfile()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(moduledir, "Containerfile"), path)
	})

	t.Run("containerfile ships the one it names", func(t *testing.T) {
		path, err := upload{Dir: dir, Name: "workload", Containerfile: filepath.Join("custom", "Containerfile")}.containerfile()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dir, "custom", "Containerfile"), path)
	})

	t.Run("the one it names wins over the nearest one", func(t *testing.T) {
		path, err := upload{Dir: dir, Name: "workload", Containerfile: filepath.Join("custom", "Containerfile")}.containerfile()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dir, "custom", "Containerfile"), path)
	})

	t.Run("a containerfile that does not exist is an error", func(t *testing.T) {
		_, err := upload{Dir: dir, Name: "workload", Containerfile: "missing"}.containerfile()
		require.ErrorContains(t, err, "unable to use the containerfile")
	})

	t.Run("no Containerfile to find ships none", func(t *testing.T) {
		path, err := upload{Dir: t.TempDir(), Name: "workload"}.containerfile()
		require.NoError(t, err)
		require.Empty(t, path)
	})
}
