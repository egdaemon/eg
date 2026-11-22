package runners_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/runners"
	"github.com/stretchr/testify/require"
)

func TestLocateContainerfile(t *testing.T) {
	repo := t.TempDir()
	for _, rel := range []string{
		".eg/dev/Containerfile",
		".eg/dev/own/Containerfile",
		".eg/dev/own/deep/main.go",
		".eg/dev/shared/main.go",
		".eg/other/main.go",
		"Containerfile", // outside the module directory, never used.
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(repo, rel)), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(repo, rel), []byte("FROM scratch\n"), 0600))
	}

	root := filepath.Join(repo, ".eg")

	t.Run("nearest containerfile wins, from the workload up to the module directory", func(t *testing.T) {
		for name, expected := range map[string]string{
			"dev/shared":   ".eg/dev/Containerfile",
			"dev/own":      ".eg/dev/own/Containerfile",
			"dev/own/deep": ".eg/dev/own/Containerfile",
			"dev":          ".eg/dev/Containerfile",
		} {
			found, err := runners.LocateContainerfile(repo, root, filepath.Join(root, name), "")
			require.NoError(t, err, name)
			require.Equal(t, filepath.Join(repo, expected), found, name)
		}
	})

	t.Run("none found is an empty path without an error", func(t *testing.T) {
		found, err := runners.LocateContainerfile(repo, root, filepath.Join(root, "other"), "")
		require.NoError(t, err)
		require.Empty(t, found)
	})

	t.Run("explicit is relative to the repository", func(t *testing.T) {
		found, err := runners.LocateContainerfile(repo, root, filepath.Join(root, "other"), ".eg/dev/Containerfile")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(repo, ".eg", "dev", "Containerfile"), found)
	})

	t.Run("explicit absolute is used as is", func(t *testing.T) {
		abs := filepath.Join(repo, "Containerfile")
		found, err := runners.LocateContainerfile(repo, root, filepath.Join(root, "other"), abs)
		require.NoError(t, err)
		require.Equal(t, abs, found)
	})

	t.Run("explicit missing is an error", func(t *testing.T) {
		_, err := runners.LocateContainerfile(repo, root, filepath.Join(root, "other"), ".eg/missing")
		require.ErrorContains(t, err, "unable to use the containerfile")
	})
}
