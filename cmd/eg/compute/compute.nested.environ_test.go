package compute

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNestedEnviron(t *testing.T) {
	t.Run("inherits the file when nested", func(t *testing.T) {
		t.Setenv("container", "podman")
		path := filepath.Join(t.TempDir(), "environ.env")
		require.NoError(t, os.WriteFile(path, []byte("INHERITED=yes\n"), 0600))

		environ, err := nestedenviron(path).Environ()
		require.NoError(t, err)
		require.Contains(t, environ, "INHERITED=yes")
	})

	t.Run("ignores the file when not nested", func(t *testing.T) {
		t.Setenv("container", "")
		path := filepath.Join(t.TempDir(), "environ.env")
		require.NoError(t, os.WriteFile(path, []byte("INHERITED=yes\n"), 0600))

		environ, err := nestedenviron(path).Environ()
		require.NoError(t, err)
		require.NotContains(t, environ, "INHERITED=yes")
	})

	t.Run("a missing file is a noop", func(t *testing.T) {
		t.Setenv("container", "podman")

		environ, err := nestedenviron(filepath.Join(t.TempDir(), "missing.env")).Environ()
		require.NoError(t, err)
		require.Empty(t, environ)
	})

	t.Run("later values override the inherited ones", func(t *testing.T) {
		t.Setenv("container", "podman")
		path := filepath.Join(t.TempDir(), "environ.env")
		require.NoError(t, os.WriteFile(path, []byte("KEY=inherited\n"), 0600))

		environ, err := nestedenviron(path).Var("KEY", "override").Environ()
		require.NoError(t, err)
		require.Equal(t, "override", environ[len(environ)-1][len("KEY="):])
	})
}
