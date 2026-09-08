package fsx_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/fsx"
	"github.com/stretchr/testify/require"
)

func TestRemoveAll(t *testing.T) {
	// readonly mirrors what the golang toolchain writes into GOMODCACHE:
	// a 0555 directory holding 0444 files.
	readonly := func(t *testing.T, dir string) string {
		t.Helper()
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package pkg"), 0444))
		require.NoError(t, os.Chmod(dir, 0555))
		t.Cleanup(func() { os.Chmod(dir, 0755) })
		return dir
	}

	mode := func(t *testing.T, path string) fs.FileMode {
		t.Helper()
		info, err := os.Stat(path)
		require.NoError(t, err)
		return info.Mode().Perm()
	}

	t.Run("removes_read_only_tree", func(t *testing.T) {
		root := t.TempDir()
		mod := filepath.Join(root, "mod")
		readonly(t, filepath.Join(mod, "example.com", "pkg@v1.0.0"))
		readonly(t, filepath.Join(mod, "example.com", "pkg@v1.1.0"))
		require.NoError(t, os.WriteFile(filepath.Join(mod, "example.com", "list"), []byte{}, 0644))
		require.Equal(t, fs.FileMode(0555), mode(t, filepath.Join(mod, "example.com", "pkg@v1.0.0")))

		require.NoError(t, fsx.RemoveAll(mod))
		require.NoDirExists(t, mod)
	})

	t.Run("removes_writable_tree", func(t *testing.T) {
		root := t.TempDir()
		tree := filepath.Join(root, "tree")
		require.NoError(t, os.MkdirAll(filepath.Join(tree, "a", "b"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(tree, "a", "b", "file"), []byte{}, 0644))

		require.NoError(t, fsx.RemoveAll(tree))
		require.NoDirExists(t, tree)
	})

	t.Run("removes_file", func(t *testing.T) {
		root := t.TempDir()
		file := filepath.Join(root, "file")
		require.NoError(t, os.WriteFile(file, []byte{}, 0444))

		require.NoError(t, fsx.RemoveAll(file))
		require.NoFileExists(t, file)
	})

	t.Run("missing_path_is_noop", func(t *testing.T) {
		require.NoError(t, fsx.RemoveAll(filepath.Join(t.TempDir(), "missing")))
	})

	t.Run("does_not_follow_symlinks", func(t *testing.T) {
		root := t.TempDir()
		target := readonly(t, filepath.Join(root, "target"))
		tree := filepath.Join(root, "tree")
		require.NoError(t, os.MkdirAll(tree, 0755))
		require.NoError(t, os.Symlink(target, filepath.Join(tree, "link")))

		require.NoError(t, fsx.RemoveAll(tree))
		require.NoDirExists(t, tree)
		require.Equal(t, fs.FileMode(0555), mode(t, target))
		require.FileExists(t, filepath.Join(target, "main.go"))
	})

	t.Run("symlink_root_removes_link_only", func(t *testing.T) {
		root := t.TempDir()
		target := readonly(t, filepath.Join(root, "target"))
		link := filepath.Join(root, "link")
		require.NoError(t, os.Symlink(target, link))

		require.NoError(t, fsx.RemoveAll(link))
		require.NoFileExists(t, link)
		require.Equal(t, fs.FileMode(0555), mode(t, target))
		require.FileExists(t, filepath.Join(target, "main.go"))
	})
}
