package egfs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
	"github.com/stretchr/testify/require"
)

func TestCloneDirectory(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	tmpdir := t.TempDir()

	require.NoError(t, egfs.CloneFS(ctx, tmpdir, ".", os.DirFS(testx.Fixture())))
	require.Equal(t, testx.ReadMD5(testx.Fixture("example.txt")), testx.ReadMD5(tmpdir, "example.txt"))
	require.Equal(t, testx.ReadMD5(testx.Fixture("dir1", "example.txt")), testx.ReadMD5(tmpdir, "dir1", "example.txt"))
}

func TestCloneFile(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	tmpdir := t.TempDir()

	require.NoError(t, egfs.CloneFS(ctx, tmpdir, filepath.Join("dir1", "example.txt"), os.DirFS(testx.Fixture())))
	require.Equal(t, testx.ReadMD5(testx.Fixture("dir1", "example.txt")), testx.ReadMD5(tmpdir, "dir1", "example.txt"))
}

func TestFileExistsFn(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	t.Run("exists", func(t *testing.T) {
		fn := egfs.FileExistsFn(testx.Fixture("example.txt"))
		require.True(t, fn(ctx))
	})

	t.Run("missing", func(t *testing.T) {
		fn := egfs.FileExistsFn(testx.Fixture("nonexistent.txt"))
		require.False(t, fn(ctx))
	})

	t.Run("directory returns false", func(t *testing.T) {
		fn := egfs.FileExistsFn(testx.Fixture("dir1"))
		require.False(t, fn(ctx))
	})
}

func TestFileNotExistsFn(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	t.Run("exists", func(t *testing.T) {
		fn := egfs.FileNotExistsFn(testx.Fixture("example.txt"))
		require.False(t, fn(ctx))
	})

	t.Run("missing", func(t *testing.T) {
		fn := egfs.FileNotExistsFn(testx.Fixture("nonexistent.txt"))
		require.True(t, fn(ctx))
	})

	t.Run("directory returns true", func(t *testing.T) {
		fn := egfs.FileNotExistsFn(testx.Fixture("dir1"))
		require.True(t, fn(ctx))
	})
}

func TestDirExistsFn(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	t.Run("exists", func(t *testing.T) {
		fn := egfs.DirExistsFn(testx.Fixture("dir1"))
		require.True(t, fn(ctx))
	})

	t.Run("missing", func(t *testing.T) {
		fn := egfs.DirExistsFn(testx.Fixture("nonexistent"))
		require.False(t, fn(ctx))
	})

	t.Run("file returns false", func(t *testing.T) {
		fn := egfs.DirExistsFn(testx.Fixture("example.txt"))
		require.False(t, fn(ctx))
	})
}

func TestDirNotExistsFn(t *testing.T) {
	ctx, done := testx.Context(t)
	defer done()

	t.Run("exists", func(t *testing.T) {
		fn := egfs.DirNotExistsFn(testx.Fixture("dir1"))
		require.False(t, fn(ctx))
	})

	t.Run("missing", func(t *testing.T) {
		fn := egfs.DirNotExistsFn(testx.Fixture("nonexistent"))
		require.True(t, fn(ctx))
	})

	t.Run("file returns true", func(t *testing.T) {
		fn := egfs.DirNotExistsFn(testx.Fixture("example.txt"))
		require.True(t, fn(ctx))
	})
}

func TestFindFirst(t *testing.T) {
	require.Equal(t, "dir1/dir2/example.txt", egfs.FindFirst(os.DirFS(testx.Fixture()), "example.txt"))
	require.Equal(t, "dir1/dir2", egfs.FindFirst(os.DirFS(testx.Fixture()), "dir2"))
}

func TestString(t *testing.T) {
	t.Run("reads the file", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "a.txt")
		require.NoError(t, os.WriteFile(p, []byte("hello"), 0600))

		got, err := egfs.String(p)
		require.NoError(t, err)
		require.Equal(t, "hello", got)
	})

	t.Run("missing file errors", func(t *testing.T) {
		got, err := egfs.String(filepath.Join(t.TempDir(), "missing.txt"))
		require.ErrorIs(t, err, os.ErrNotExist)
		require.Empty(t, got)
	})
}

func TestStringLazy(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	read := egfs.StringLazy(p)

	_, err := read()
	require.ErrorIs(t, err, os.ErrNotExist, "nothing is read until called")

	require.NoError(t, os.WriteFile(p, []byte("hello"), 0600))
	got, err := read()
	require.NoError(t, err)
	require.Equal(t, "hello", got)
}
