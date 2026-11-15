package compute

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServeContainerfile(t *testing.T) {
	t.Run("nearest containerfile wins, from the workload up to the module directory", func(t *testing.T) {
		root := t.TempDir()
		for _, rel := range []string{
			".eg/dev/Containerfile",
			".eg/dev/own/Containerfile",
			".eg/dev/own/deep/main.go",
			".eg/dev/shared/main.go",
			".eg/other/main.go",
			"Containerfile", // outside the module directory, never used.
		} {
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0700))
			require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte("FROM scratch\n"), 0600))
		}

		for name, expected := range map[string]string{
			"dev/shared":   ".eg/dev/Containerfile",
			"dev/own":      ".eg/dev/own/Containerfile",
			"dev/own/deep": ".eg/dev/own/Containerfile",
			"dev":          ".eg/dev/Containerfile",
		} {
			found, err := serve{Dir: root, ModuleDir: ".eg", Name: name}.containerfile()
			require.NoError(t, err, name)
			require.Equal(t, filepath.Join(root, expected), found, name)
		}

		// nothing between the workload and the module directory.
		_, err := serve{Dir: root, ModuleDir: ".eg", Name: "other"}.containerfile()
		require.ErrorContains(t, err, "no Containerfile")
	})

	t.Run("the containerfile flag is relative to the repository directory", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".eg", "dev"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(root, ".eg", "dev", "Containerfile"), []byte("FROM scratch\n"), 0600))

		found, err := serve{Dir: root, ModuleDir: ".eg", Name: "elsewhere", Containerfile: ".eg/dev/Containerfile"}.containerfile()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(root, ".eg", "dev", "Containerfile"), found)

		_, err = serve{Dir: root, ModuleDir: ".eg", Name: "x", Containerfile: ".eg/missing"}.containerfile()
		require.Error(t, err)
	})
}
