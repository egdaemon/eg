package egiso

import (
	"io/fs"
	"os"
	"path"
	"syscall"

	"golang.org/x/sys/unix"
)

// Mknod makes the named socket or fifo, with mode as a stat's st_mode, in its Bind mount's host
// directory, as a process binding a unix socket there makes one. The upper holds neither.
func (t *Upper) Mknod(name string, mode uint32) (err error) {
	var (
		dir *os.File
	)

	m, rel, ok := t.bound(name)
	if kind := mode & syscall.S_IFMT; !ok || (kind != syscall.S_IFSOCK && kind != syscall.S_IFIFO) {
		return &fs.PathError{Op: "mknod", Path: name, Err: syscall.EOPNOTSUPP}
	}
	if dir, err = m.root.Open(path.Dir(rel)); err != nil {
		return err
	}
	defer dir.Close()
	return unix.Mknodat(int(dir.Fd()), path.Base(rel), mode, 0)
}
