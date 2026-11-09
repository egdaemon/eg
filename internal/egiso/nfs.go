package egiso

import (
	"hash/fnv"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/go-git/go-billy/v5"
	nfs "github.com/willscott/go-nfs"
	nfsfile "github.com/willscott/go-nfs/file"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

// ServeNFS serves fsys over NFSv3 on l until l closes, so native processes on the host can use the
// upper as a real filesystem by mounting it, without a kernel extension: macOS and Linux mount NFS
// themselves. Every file belongs to the host user, which the mounting processes run as.
func ServeNFS(l net.Listener, fsys *SysFS) error {
	return nfs.Serve(l, nfshelper.NewCachingHandler(nfshelper.NewNullAuthHandler(nfsfs{sys: fsys}), 4096))
}

// nfsfs is the billy filesystem go-nfs serves. NFS clients resolve symlinks themselves, so the
// paths it receives are the upper's own; only Stat and Open follow a final symlink.
type nfsfs struct {
	sys *SysFS
}

func (t nfsfs) Create(name string) (billy.File, error) {
	return t.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

func (t nfsfs) Open(name string) (billy.File, error) {
	return t.OpenFile(name, os.O_RDONLY, 0)
}

func (t nfsfs) OpenFile(name string, flag int, perm fs.FileMode) (billy.File, error) {
	resolved, err := t.resolve(name, true)
	if err != nil {
		return nil, err
	}

	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		f, err := t.sys.upper.OpenFile(resolved, flag, perm, t.sys.user, t.sys.group)
		if err != nil {
			return nil, err
		}
		return nfshandle{name: name, f: f}, nil
	}

	if dir, rel, ok := t.sys.upper.host(resolved); ok {
		f, err := dir.Open(rel)
		if err != nil {
			return nil, err
		}
		return nfshandle{name: name, f: f}, nil
	}

	f, err := t.sys.upper.tree.Open(resolved)
	if err != nil {
		return nil, err
	}
	return nfshandle{name: name, f: f}, nil
}

func (t nfsfs) Stat(name string) (fs.FileInfo, error) {
	resolved, err := t.resolve(name, true)
	if err != nil {
		return nil, err
	}
	return t.lstat(resolved)
}

func (t nfsfs) Lstat(name string) (fs.FileInfo, error) {
	return t.lstat(clean(name))
}

func (t nfsfs) Rename(from string, to string) error {
	return t.sys.upper.Rename(clean(from), clean(to))
}

func (t nfsfs) Remove(name string) error {
	return t.sys.upper.Remove(clean(name))
}

func (t nfsfs) Join(elem ...string) string {
	return path.Join(elem...)
}

func (t nfsfs) TempFile(dir string, prefix string) (billy.File, error) {
	return nil, billy.ErrNotSupported
}

func (t nfsfs) ReadDir(name string) ([]fs.FileInfo, error) {
	name = clean(name)
	entries, err := t.sys.upper.ReadDir(name)
	if err != nil {
		return nil, err
	}

	infos := make([]fs.FileInfo, 0, len(entries))
	for _, e := range entries {
		st, err := t.lstat(path.Join(name, e.Name()))
		if err != nil {
			return nil, err
		}
		infos = append(infos, st)
	}
	return infos, nil
}

func (t nfsfs) MkdirAll(name string, perm fs.FileMode) error {
	name = clean(name)
	if st, err := t.sys.upper.Lstat(name); err == nil && st.IsDir() {
		return nil
	}
	if name != "." {
		if err := t.MkdirAll(path.Dir(name), perm); err != nil {
			return err
		}
	}
	return t.sys.upper.Mkdir(name, perm, t.sys.user, t.sys.group)
}

func (t nfsfs) Symlink(target string, link string) error {
	return t.sys.upper.Symlink(target, clean(link), t.sys.user, t.sys.group)
}

func (t nfsfs) Readlink(name string) (string, error) {
	return t.sys.upper.ReadLink(clean(name))
}

func (t nfsfs) Chroot(string) (billy.Filesystem, error) {
	return nil, billy.ErrNotSupported
}

func (t nfsfs) Root() string {
	return "/"
}

func (t nfsfs) Chmod(name string, mode fs.FileMode) error {
	return t.sys.upper.Chmod(clean(name), mode)
}

// Lchown does nothing: every file belongs to the host user.
func (t nfsfs) Lchown(name string, uid int, gid int) error {
	return nil
}

// Chown does nothing: every file belongs to the host user.
func (t nfsfs) Chown(name string, uid int, gid int) error {
	return nil
}

func (t nfsfs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return t.sys.upper.Chtimes(clean(name), atime, mtime)
}

func (t nfsfs) resolve(name string, follow bool) (string, error) {
	resolved, errno := t.sys.resolve(clean(name), follow)
	if errno != 0 {
		return "", &fs.PathError{Op: "resolve", Path: name, Err: errno}
	}
	return resolved, nil
}

// lstat describes name as owned by the host user, with an id derived from its path, which NFS
// clients use to tell files apart.
func (t nfsfs) lstat(name string) (fs.FileInfo, error) {
	st, err := t.sys.upper.Lstat(name)
	if err != nil {
		return nil, err
	}

	id := fnv.New64a()
	id.Write([]byte(name))
	links := uint32(1)
	if st.IsDir() {
		links = 2
	}
	return nfsinfo{FileInfo: st, sys: &nfsfile.FileInfo{Nlink: links, UID: uint32(os.Getuid()), GID: uint32(os.Getgid()), Fileid: id.Sum64()}}, nil
}

// clean turns a path go-nfs joined, "" for the root, into a name of the upper.
func clean(name string) string {
	if cleaned := path.Clean("/" + name); cleaned != "/" {
		return cleaned[1:]
	}
	return "."
}

type nfsinfo struct {
	fs.FileInfo
	sys *nfsfile.FileInfo
}

func (t nfsinfo) Sys() any { return t.sys }

// nfshandle is a file go-nfs reads or writes. A file of an archive reads at an offset by seeking.
type nfshandle struct {
	name string
	f    fs.File
}

func (t nfshandle) Name() string { return t.name }

func (t nfshandle) Read(p []byte) (int, error) { return t.f.Read(p) }

func (t nfshandle) ReadAt(p []byte, off int64) (n int, err error) {
	if ra, ok := t.f.(io.ReaderAt); ok {
		return ra.ReadAt(p, off)
	}
	if _, err = t.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	if n, err = io.ReadFull(t.f, p); errorsx.Is(err, io.ErrUnexpectedEOF) {
		return n, io.EOF
	}
	return n, err
}

func (t nfshandle) Write(p []byte) (int, error) {
	if w, ok := t.f.(io.Writer); ok {
		return w.Write(p)
	}
	return 0, &fs.PathError{Op: "write", Path: t.name, Err: fs.ErrPermission}
}

func (t nfshandle) Seek(offset int64, whence int) (int64, error) {
	if s, ok := t.f.(io.Seeker); ok {
		return s.Seek(offset, whence)
	}
	return 0, &fs.PathError{Op: "seek", Path: t.name, Err: errorsx.String("unseekable")}
}

func (t nfshandle) Truncate(size int64) error {
	if f, ok := t.f.(interface{ Truncate(int64) error }); ok {
		return f.Truncate(size)
	}
	return &fs.PathError{Op: "truncate", Path: t.name, Err: fs.ErrPermission}
}

func (t nfshandle) Close() error { return t.f.Close() }

func (t nfshandle) Lock() error { return nil }

func (t nfshandle) Unlock() error { return nil }
