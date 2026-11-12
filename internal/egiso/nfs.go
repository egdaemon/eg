package egiso

import (
	"encoding/binary"
	"hash/fnv"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/go-git/go-billy/v5"
	nfs "github.com/willscott/go-nfs"
	nfsfile "github.com/willscott/go-nfs/file"
	nfshelper "github.com/willscott/go-nfs/helpers"
)

// NFSOption configures ServeNFS.
type NFSOption func(*nfsfs)

// NFSOptionAccounts reports a file as owned by the ids that users and groups give the names of its
// owners, for a client that is the machine the root belongs to, rather than by the host user. An
// owner they do not name is reported as nobody. Changing a file's owner to an id they name changes
// it in the upper.
func NFSOptionAccounts(users []Account, groups []Account) NFSOption {
	return func(t *nfsfs) {
		t.users, t.groups = newids(users), newids(groups)
	}
}

// ServeNFS serves fsys over NFSv3 on l until l closes, so native processes can use the upper as a
// real filesystem by mounting it, without a kernel extension: macOS and Linux mount NFS themselves.
// By default every file belongs to the host user, which the mounting processes run as.
func ServeNFS(l net.Listener, fsys *SysFS, options ...NFSOption) error {
	served := nfsfs{sys: fsys}
	for _, opt := range options {
		opt(&served)
	}
	return nfs.Serve(l, nfshelper.NewCachingHandler(nfshelper.NewNullAuthHandler(served), 4096))
}

// nfsfs is the billy filesystem go-nfs serves. NFS clients resolve symlinks themselves, so the
// paths it receives are the upper's own; only Open follows a final symlink.
type nfsfs struct {
	sys    *SysFS
	users  *ids
	groups *ids
}

// nobody is the id of an owner the accounts do not name.
const nobody = 65534

// ids are the ids of the names of accounts, and the names of the ids.
type ids struct {
	byname map[string]uint32
	byid   map[uint32]string
}

func newids(accounts []Account) *ids {
	named := &ids{byname: make(map[string]uint32, len(accounts)), byid: make(map[uint32]string, len(accounts))}
	for _, a := range accounts {
		named.byname[a.Name] = uint32(a.ID)
		named.byid[uint32(a.ID)] = a.Name
	}
	return named
}

func (t *ids) id(name string) uint32 {
	if id, ok := t.byname[name]; ok {
		return id
	}
	return nobody
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

// Stat describes name without following a final symlink. go-nfs asks it whether a name a client
// creates already exists, and a symlink whose target is missing, as /etc/mtab's /proc/mounts is in
// an image, still holds its name.
func (t nfsfs) Stat(name string) (fs.FileInfo, error) {
	resolved, err := t.resolve(name, false)
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

// Link makes link another name of the file at path. Only FUSE calls it: go-nfs links only through a
// filesystem that also makes devices, fifos and sockets.
func (t nfsfs) Link(path string, link string) error {
	return t.sys.upper.Link(clean(path), clean(link))
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

// Lchown changes the owners of name to the accounts' uid and gid. It does nothing without accounts:
// every file belongs to the host user.
func (t nfsfs) Lchown(name string, uid int, gid int) error {
	if t.users == nil {
		return nil
	}
	return t.chown(clean(name), uid, gid)
}

// Chown changes the owners of what name resolves to, as Lchown does.
func (t nfsfs) Chown(name string, uid int, gid int) error {
	if t.users == nil {
		return nil
	}
	resolved, err := t.resolve(name, true)
	if err != nil {
		return err
	}
	return t.chown(resolved, uid, gid)
}

func (t nfsfs) chown(name string, uid int, gid int) error {
	user, uok := t.users.byid[uint32(uid)]
	group, gok := t.groups.byid[uint32(gid)]
	if !uok || !gok {
		return &fs.PathError{Op: "chown", Path: name, Err: syscall.EINVAL}
	}
	return t.sys.upper.Chown(name, user, group)
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

// lstat describes name as owned by the host user, or its owners' ids among the accounts, with an
// id derived from its path, which NFS clients use to tell files apart.
func (t nfsfs) lstat(name string) (fs.FileInfo, error) {
	var (
		uid   = uint32(os.Getuid())
		gid   = uint32(os.Getgid())
		user  string
		group string
	)

	st, err := t.sys.upper.Lstat(name)
	if err != nil {
		return nil, err
	}

	if t.users != nil {
		if user, group, err = t.sys.upper.Owner(name); err != nil {
			return nil, err
		}
		uid, gid = t.users.id(user), t.groups.id(group)
	}

	id := fnv.New64a()
	links := uint32(1)
	if st.IsDir() {
		links = 2
	}

	// a file the host holds with more than one name is named by its inode, so its names are one file,
	// with its count of names. Everything else is named by its path, which a copy up keeps, so a
	// directory stays the one the kernel mounted over.
	if host, ok := st.Sys().(*syscall.Stat_t); ok && !st.IsDir() && host.Nlink > 1 {
		binary.Write(id, binary.LittleEndian, [2]uint64{uint64(host.Dev), host.Ino})
		links = uint32(host.Nlink)
	} else {
		id.Write([]byte(name))
	}

	return nfsinfo{FileInfo: st, sys: &nfsfile.FileInfo{Nlink: links, UID: uid, GID: gid, Fileid: id.Sum64()}}, nil
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
