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

func TestFindFirst(t *testing.T) {
	require.Equal(t, "dir1/dir2/example.txt", egfs.FindFirst(os.DirFS(testx.Fixture()), "example.txt"))
	require.Equal(t, "dir1/dir2", egfs.FindFirst(os.DirFS(testx.Fixture()), "dir2"))
}
