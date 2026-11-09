package egiso

import (
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/experimental/sysfs"
	"github.com/tetratelabs/wazero/sys"
)

// SysFS serves an Upper to a WASI guest as its writable filesystem, for a wazero FSConfig's
// WithSysFSMount. WASI has no owners, so everything the guest creates belongs to user and group.
// Symlinks resolve across the whole upper, an absolute target from its root, the way a kernel
// resolves them in a root filesystem.
type SysFS struct {
	experimentalsys.UnimplementedFS
	upper *Upper
	user  string
	group string
}

// NewSysFS serves upper to a guest whose writes are owned by user and group.
func NewSysFS(upper *Upper, user string, group string) *SysFS {
	return &SysFS{upper: upper, user: user, group: group}
}

// OpenFile opens the named file. A file opened only for reading is read from wherever it lives; one
// opened for writing is copied up or created first.
func (t *SysFS) OpenFile(name string, flag experimentalsys.Oflag, perm fs.FileMode) (experimentalsys.File, experimentalsys.Errno) {
	resolved, errno := t.resolve(name, flag&experimentalsys.O_NOFOLLOW == 0)
	if errno != 0 {
		return nil, errno
	}

	writes := flag&(experimentalsys.O_RDWR|experimentalsys.O_WRONLY|experimentalsys.O_CREAT|experimentalsys.O_TRUNC|experimentalsys.O_APPEND) != 0
	st, err := t.upper.Lstat(resolved)
	switch {
	case err == nil && flag&experimentalsys.O_CREAT != 0 && flag&experimentalsys.O_EXCL != 0:
		return nil, experimentalsys.EEXIST
	case err == nil && st.Mode()&fs.ModeSymlink != 0:
		return nil, experimentalsys.ELOOP
	case err == nil && st.IsDir() && writes:
		return nil, experimentalsys.EISDIR
	case err == nil && st.IsDir():
		return &dirfile{fsys: t, name: resolved}, 0
	case err == nil && flag&experimentalsys.O_DIRECTORY != 0:
		return nil, experimentalsys.ENOTDIR
	case err != nil && (!errorsx.Is(err, fs.ErrNotExist) || flag&experimentalsys.O_CREAT == 0):
		return nil, errnoof(err)
	}

	if !writes {
		if dir, rel, ok := t.upper.host(resolved); ok {
			f, errno := sysfs.DirFS(dir.Name()).OpenFile(rel, flag, 0)
			return t.virtual(f, resolved), errno
		}
		return (&sysfs.AdaptFS{FS: t.upper.tree}).OpenFile(resolved, flag, 0)
	}

	dir, rel, _, err := t.upper.prepare(resolved, osflags(flag), perm, t.user, t.group)
	if err != nil {
		return nil, errnoof(err)
	}
	if dir == t.upper.dir {
		flag &^= experimentalsys.O_CREAT | experimentalsys.O_EXCL
	}
	f, errno := sysfs.DirFS(dir.Name()).OpenFile(rel, flag, perm)
	return t.virtual(f, resolved), errno
}

// Lstat describes the named file without following a final symlink.
func (t *SysFS) Lstat(name string) (sys.Stat_t, experimentalsys.Errno) {
	return t.stat(name, false)
}

// Stat describes the named file.
func (t *SysFS) Stat(name string) (sys.Stat_t, experimentalsys.Errno) {
	return t.stat(name, true)
}

// Readlink returns the target of the named symlink.
func (t *SysFS) Readlink(name string) (string, experimentalsys.Errno) {
	resolved, errno := t.resolve(name, false)
	if errno != 0 {
		return "", errno
	}
	target, err := t.upper.ReadLink(resolved)
	return target, errnoof(err)
}

// Mkdir creates the named directory.
func (t *SysFS) Mkdir(name string, perm fs.FileMode) experimentalsys.Errno {
	resolved, errno := t.resolve(name, false)
	if errno != 0 {
		return errno
	}
	return errnoof(t.upper.Mkdir(resolved, perm, t.user, t.group))
}

// Chmod sets the named file's mode bits.
func (t *SysFS) Chmod(name string, perm fs.FileMode) experimentalsys.Errno {
	resolved, errno := t.resolve(name, true)
	if errno != 0 {
		return errno
	}
	return errnoof(t.upper.Chmod(resolved, perm))
}

// Rename renames from to to.
func (t *SysFS) Rename(from string, to string) experimentalsys.Errno {
	src, errno := t.resolve(from, false)
	if errno != 0 {
		return errno
	}
	dst, errno := t.resolve(to, false)
	if errno != 0 {
		return errno
	}
	return errnoof(t.upper.Rename(src, dst))
}

// Rmdir removes the named empty directory.
func (t *SysFS) Rmdir(name string) experimentalsys.Errno {
	return t.remove(name, true)
}

// Unlink removes the named file.
func (t *SysFS) Unlink(name string) experimentalsys.Errno {
	return t.remove(name, false)
}

// Symlink creates the symlink link to target.
func (t *SysFS) Symlink(target string, link string) experimentalsys.Errno {
	resolved, errno := t.resolve(link, false)
	if errno != 0 {
		return errno
	}
	return errnoof(t.upper.Symlink(target, resolved, t.user, t.group))
}

// Utimens sets the named file's access and modification times, in epoch nanoseconds;
// experimentalsys.UTIME_OMIT leaves one unchanged.
func (t *SysFS) Utimens(name string, atim int64, mtim int64) experimentalsys.Errno {
	resolved, errno := t.resolve(name, true)
	if errno != 0 {
		return errno
	}
	return t.utimens(resolved, atim, mtim)
}

func (t *SysFS) stat(name string, follow bool) (sys.Stat_t, experimentalsys.Errno) {
	resolved, errno := t.resolve(name, follow)
	if errno != 0 {
		return sys.Stat_t{}, errno
	}
	st, err := t.upper.Lstat(resolved)
	if err != nil {
		return sys.Stat_t{}, errnoof(err)
	}
	return statof(st), 0
}

func (t *SysFS) remove(name string, dir bool) experimentalsys.Errno {
	resolved, errno := t.resolve(name, false)
	if errno != 0 {
		return errno
	}
	st, err := t.upper.Lstat(resolved)
	switch {
	case err != nil:
		return errnoof(err)
	case dir && !st.IsDir():
		return experimentalsys.ENOTDIR
	case !dir && st.IsDir():
		return experimentalsys.EISDIR
	}
	return errnoof(t.upper.Remove(resolved))
}

func (t *SysFS) utimens(name string, atim int64, mtim int64) experimentalsys.Errno {
	st, err := t.upper.Lstat(name)
	if err != nil {
		return errnoof(err)
	}
	current := statof(st)
	if atim == experimentalsys.UTIME_OMIT {
		atim = current.Atim
	}
	if mtim == experimentalsys.UTIME_OMIT {
		mtim = current.Mtim
	}
	return errnoof(t.upper.Chtimes(name, time.Unix(0, atim), time.Unix(0, mtim)))
}

// resolve returns name with the symlinks in it resolved, its final component's only when follow is
// set. A final component that does not exist is left for the caller to create.
func (t *SysFS) resolve(name string, follow bool) (string, experimentalsys.Errno) {
	var (
		resolved = "."
		hops     int
		pending  = strings.Split(path.Clean(name), "/")
	)

	follow = follow || strings.HasSuffix(name, "/")
	for len(pending) > 0 {
		component := pending[0]
		pending = pending[1:]

		switch component {
		case ".", "":
			continue
		case "..":
			resolved = path.Dir(resolved)
			continue
		}

		next := path.Join(resolved, component)
		final := len(pending) == 0
		if final && !follow {
			resolved = next
			continue
		}

		st, err := t.upper.Lstat(next)
		if final && errorsx.Is(err, fs.ErrNotExist) {
			resolved = next
			continue
		}
		if err != nil {
			return "", errnoof(err)
		}
		if st.Mode()&fs.ModeSymlink == 0 {
			resolved = next
			continue
		}

		if hops++; hops > maxsymlinks {
			return "", experimentalsys.ELOOP
		}
		target, err := t.upper.ReadLink(next)
		if err != nil {
			return "", errnoof(err)
		}
		if path.IsAbs(target) {
			resolved = "."
		}
		pending = append(strings.Split(target, "/"), pending...)
	}

	return resolved, 0
}

// virtual describes a file the guest opened from a host directory as the upper does.
func (t *SysFS) virtual(f experimentalsys.File, name string) experimentalsys.File {
	if f == nil {
		return nil
	}
	return &virtualfile{File: f, fsys: t, name: name}
}

type virtualfile struct {
	experimentalsys.File
	fsys *SysFS
	name string
}

func (t *virtualfile) Stat() (sys.Stat_t, experimentalsys.Errno) {
	return t.fsys.stat(t.name, false)
}

// dirfile is a directory of the upper, listed through it so the tree's entries appear.
type dirfile struct {
	experimentalsys.DirFile
	fsys    *SysFS
	name    string
	pending []experimentalsys.Dirent
	listed  bool
}

func (t *dirfile) Dev() (uint64, experimentalsys.Errno) {
	st, errno := t.Stat()
	return st.Dev, errno
}

func (t *dirfile) Ino() (sys.Inode, experimentalsys.Errno) {
	st, errno := t.Stat()
	return st.Ino, errno
}

func (t *dirfile) Stat() (sys.Stat_t, experimentalsys.Errno) {
	return t.fsys.stat(t.name, false)
}

// Seek rewinds the listing; a directory has no other offset.
func (t *dirfile) Seek(offset int64, whence int) (int64, experimentalsys.Errno) {
	if offset != 0 || whence != io.SeekStart {
		return 0, experimentalsys.EINVAL
	}
	t.pending, t.listed = nil, false
	return 0, 0
}

// Readdir lists the directory n entries at a time, or all of the rest when n is not positive.
func (t *dirfile) Readdir(n int) ([]experimentalsys.Dirent, experimentalsys.Errno) {
	if !t.listed {
		entries, err := t.fsys.upper.ReadDir(t.name)
		if err != nil {
			return nil, errnoof(err)
		}
		t.pending = make([]experimentalsys.Dirent, 0, len(entries))
		for _, e := range entries {
			t.pending = append(t.pending, experimentalsys.Dirent{Name: e.Name(), Type: e.Type()})
		}
		t.listed = true
	}

	if n <= 0 || n > len(t.pending) {
		n = len(t.pending)
	}
	entries := t.pending[:n]
	t.pending = t.pending[n:]
	return entries, 0
}

func (t *dirfile) Sync() experimentalsys.Errno { return 0 }

func (t *dirfile) Datasync() experimentalsys.Errno { return 0 }

func (t *dirfile) Utimens(atim int64, mtim int64) experimentalsys.Errno {
	return t.fsys.utimens(t.name, atim, mtim)
}

func (t *dirfile) Close() experimentalsys.Errno { return 0 }

// statof describes a file with the mode the upper gives it.
func statof(info fs.FileInfo) sys.Stat_t {
	st := sys.NewStat_t(info)
	st.Mode = info.Mode()
	return st
}

// osflags converts the flags Upper.prepare acts on.
func osflags(flag experimentalsys.Oflag) (converted int) {
	switch {
	case flag&experimentalsys.O_RDWR != 0:
		converted = syscall.O_RDWR
	case flag&experimentalsys.O_WRONLY != 0:
		converted = syscall.O_WRONLY
	}
	if flag&experimentalsys.O_CREAT != 0 {
		converted |= syscall.O_CREAT
	}
	if flag&experimentalsys.O_EXCL != 0 {
		converted |= syscall.O_EXCL
	}
	return converted
}

// errnoof maps an error, however it is wrapped, to the errno WASI reports.
func errnoof(err error) experimentalsys.Errno {
	var (
		errno syscall.Errno
	)

	switch {
	case err == nil:
		return 0
	case errors.As(err, &errno):
		return experimentalsys.UnwrapOSError(errno)
	case errorsx.Is(err, fs.ErrNotExist):
		return experimentalsys.ENOENT
	case errorsx.Is(err, fs.ErrExist):
		return experimentalsys.EEXIST
	case errorsx.Is(err, fs.ErrInvalid):
		return experimentalsys.EINVAL
	case errorsx.Is(err, fs.ErrPermission):
		return experimentalsys.EPERM
	default:
		return experimentalsys.UnwrapOSError(err)
	}
}
