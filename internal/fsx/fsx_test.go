package fsx_test

import (
	"context"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"syscall"
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

func TestRelative(t *testing.T) {
	rel, ok := fsx.Relative("/a/b", "/a/b/c/d")
	require.True(t, ok)
	require.Equal(t, "c/d", rel)

	rel, ok = fsx.Relative("/a/b", "/a/b")
	require.True(t, ok)
	require.Equal(t, ".", rel)

	for _, path := range []string{"/a/b/../c", "/a/b-other/x", "/a", "/x", "c/d"} {
		_, ok = fsx.Relative("/a/b", path)
		require.False(t, ok, path)
	}
}

func TestCloneTree(t *testing.T) {
	type file struct {
		name     string
		contents string
	}

	// source is a directory, .gnupg, holding a file, a dotfile and a nested file, beside dir's other
	// contents. Its path is short enough to bind a unix socket in, which macOS limits to 104 bytes.
	source := func(t *testing.T) string {
		dir, err := os.MkdirTemp("/tmp", "clonetree")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(dir) })
		src := filepath.Join(dir, ".gnupg")
		require.NoError(t, os.MkdirAll(filepath.Join(src, "private-keys-v1.d"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(src, "pubring.kbx"), []byte("pubring"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(src, ".#lk0x1"), []byte("lock"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(src, "private-keys-v1.d", "key"), []byte("key"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "outside"), []byte("outside"), 0600))
		return src
	}

	t.Run("copies the directory's files", func(t *testing.T) {
		src := source(t)
		dst := filepath.Join(t.TempDir(), "copy")
		require.NoError(t, fsx.CloneTree(t.Context(), dst, filepath.Base(src), os.DirFS(filepath.Dir(src))))

		for _, f := range []file{{"pubring.kbx", "pubring"}, {".#lk0x1", "lock"}, {filepath.Join("private-keys-v1.d", "key"), "key"}} {
			b, err := os.ReadFile(filepath.Join(dst, f.name))
			require.NoError(t, err, f.name)
			require.Equal(t, f.contents, string(b), f.name)
		}

		_, err := os.Stat(filepath.Join(dst, "outside"))
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("skips sockets", func(t *testing.T) {
		src := source(t)
		l, err := net.Listen("unix", filepath.Join(src, "S.gpg-agent"))
		require.NoError(t, err)
		defer l.Close()

		dst := filepath.Join(t.TempDir(), "copy")
		require.NoError(t, fsx.CloneTree(t.Context(), dst, filepath.Base(src), os.DirFS(filepath.Dir(src))))

		_, err = os.Lstat(filepath.Join(dst, "S.gpg-agent"))
		require.ErrorIs(t, err, fs.ErrNotExist)
		_, err = os.Stat(filepath.Join(dst, "pubring.kbx"))
		require.NoError(t, err)
	})

	t.Run("skips named pipes", func(t *testing.T) {
		src := source(t)
		require.NoError(t, syscall.Mkfifo(filepath.Join(src, "fifo"), 0600))

		dst := filepath.Join(t.TempDir(), "copy")
		require.NoError(t, fsx.CloneTree(t.Context(), dst, filepath.Base(src), os.DirFS(filepath.Dir(src))))

		_, err := os.Lstat(filepath.Join(dst, "fifo"))
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("stops when the context is cancelled", func(t *testing.T) {
		src := source(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, fsx.CloneTree(ctx, filepath.Join(t.TempDir(), "copy"), filepath.Base(src), os.DirFS(filepath.Dir(src))), context.Canceled)
	})
}
