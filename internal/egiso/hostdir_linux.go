package egiso

import (
	"io/fs"
	"os"

	"golang.org/x/sys/unix"
)

// Lstat describes name without following a final symlink. openat2 resolves it beneath the directory,
// as os.Root does, in one call; kernels before openat2 fall back to os.Root.
func (t hostdir) Lstat(name string) (fs.FileInfo, error) {
	var (
		dir *os.File
		fd  int
		err error
	)

	if dir, err = t.self(); err != nil {
		return nil, err
	}

	how := unix.OpenHow{Flags: unix.O_PATH | unix.O_NOFOLLOW | unix.O_CLOEXEC, Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS}
	if fd, err = unix.Openat2(int(dir.Fd()), name, &how); err == unix.ENOSYS {
		return t.Root.Lstat(name)
	} else if err != nil {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: err}
	}

	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	return f.Stat()
}
