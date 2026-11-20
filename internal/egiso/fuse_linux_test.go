package egiso

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/stretchr/testify/require"
)

func TestFusefileGetattr(t *testing.T) {
	opened := func(t *testing.T, contents string) (*fusefile, *os.File) {
		path := filepath.Join(t.TempDir(), "unlinked")
		require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		require.NoError(t, err)
		t.Cleanup(func() { f.Close() })
		require.NoError(t, os.Remove(path))
		return &fusefile{f: nfshandle{name: "unlinked", f: f}, attr: fuse.Attr{Ino: 7, Mode: syscall.S_IFREG | 0600, Nlink: 1, Owner: fuse.Owner{Uid: 5, Gid: 6}}}, f
	}

	t.Run("an unlinked file is described as opened with its size", func(t *testing.T) {
		var out fuse.AttrOut
		h, _ := opened(t, "hello")

		require.Equal(t, syscall.Errno(0), h.Getattr(t.Context(), &out))
		require.Equal(t, uint64(7), out.Ino)
		require.Equal(t, uint32(syscall.S_IFREG|0600), out.Mode)
		require.Equal(t, uint32(5), out.Uid)
		require.Equal(t, uint32(6), out.Gid)
		require.Equal(t, uint32(0), out.Nlink)
		require.Equal(t, uint64(5), out.Size)
	})

	t.Run("an unlinked file's size follows what its handles write", func(t *testing.T) {
		var out fuse.AttrOut
		h, f := opened(t, "hello")

		_, err := f.WriteAt([]byte(" world"), 5)
		require.NoError(t, err)
		require.Equal(t, syscall.Errno(0), h.Getattr(t.Context(), &out))
		require.Equal(t, uint64(11), out.Size)
		require.Equal(t, uint64(1), out.Blocks)
	})

	t.Run("a closed file fails", func(t *testing.T) {
		var out fuse.AttrOut
		h, f := opened(t, "hello")

		require.NoError(t, f.Close())
		require.NotEqual(t, syscall.Errno(0), h.Getattr(t.Context(), &out))
	})
}
