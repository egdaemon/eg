package egiso

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/go-git/go-billy/v5"
	gofs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	nfsfile "github.com/willscott/go-nfs/file"
	"golang.org/x/sys/unix"
)

// ServeFUSE serves fsys on dev, a /dev/fuse descriptor another process already mounted, until the
// mount goes away. A process in a user namespace can mount FUSE, where it can not mount NFS. Files
// belong to the ids ServeNFS reports for the same options, and a file a process creates belongs to
// it, as the kernel reports the process's ids within the namespace that mounted dev. Only the mount
// changes fsys while it is served, so the kernel keeps what it learns of it.
func ServeFUSE(dev *os.File, fsys *SysFS, options ...NFSOption) error {
	var (
		served  = nfsfs{sys: fsys}
		server  *fuse.Server
		fd      int
		err     error
		forever = time.Hour
	)

	for _, opt := range options {
		opt(&served)
	}

	// the server closes the descriptor it serves when the mount goes away, so it serves its own.
	if fd, err = unix.FcntlInt(dev.Fd(), unix.F_DUPFD_CLOEXEC, 0); err != nil {
		return errorsx.Wrap(err, "unable to duplicate the mount's descriptor")
	}
	if server, err = fuse.NewServer(gofs.NewNodeFS(&fusenode{fsys: served}, &gofs.Options{EntryTimeout: &forever, AttrTimeout: &forever, NegativeTimeout: &forever}), fmt.Sprintf("/dev/fd/%d", fd), &fuse.MountOptions{DisableXAttrs: true}); err != nil {
		return errorsx.Compact(errorsx.Wrap(err, "unable to serve the mount"), unix.Close(fd))
	}
	server.Serve()
	return nil
}

// fusenode is a file of the upper, named by its path from the root.
type fusenode struct {
	gofs.Inode
	fsys nfsfs
}

var (
	_ gofs.NodeGetattrer  = (*fusenode)(nil)
	_ gofs.NodeSetattrer  = (*fusenode)(nil)
	_ gofs.NodeLookuper   = (*fusenode)(nil)
	_ gofs.NodeReaddirer  = (*fusenode)(nil)
	_ gofs.NodeOpener     = (*fusenode)(nil)
	_ gofs.NodeCreater    = (*fusenode)(nil)
	_ gofs.NodeMkdirer    = (*fusenode)(nil)
	_ gofs.NodeUnlinker   = (*fusenode)(nil)
	_ gofs.NodeRmdirer    = (*fusenode)(nil)
	_ gofs.NodeRenamer    = (*fusenode)(nil)
	_ gofs.NodeSymlinker  = (*fusenode)(nil)
	_ gofs.NodeReadlinker = (*fusenode)(nil)
	_ gofs.NodeStatfser   = (*fusenode)(nil)
	_ gofs.NodeFsyncer    = (*fusenode)(nil)
)

func (t *fusenode) name() string {
	return clean(t.Path(nil))
}

func (t *fusenode) child(name string) string {
	return path.Join(t.name(), name)
}

// entry describes the file at name in out, and returns its inode.
func (t *fusenode) entry(ctx context.Context, name string, out *fuse.EntryOut) (*gofs.Inode, syscall.Errno) {
	info, err := t.fsys.Lstat(name)
	if err != nil {
		return nil, errnoofuse(err)
	}
	attrof(info, &out.Attr)
	return t.NewInode(ctx, &fusenode{fsys: t.fsys}, gofs.StableAttr{Mode: out.Attr.Mode & syscall.S_IFMT, Ino: out.Attr.Ino}), 0
}

// owned gives name to the process creating it, as an NFS client's create does.
func (t *fusenode) owned(ctx context.Context, name string) syscall.Errno {
	caller, ok := fuse.FromContext(ctx)
	if !ok {
		return 0
	}
	return errnoofuse(t.fsys.Lchown(name, int(caller.Uid), int(caller.Gid)))
}

func (t *fusenode) Getattr(ctx context.Context, f gofs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	info, err := t.fsys.Lstat(t.name())
	if err != nil {
		return errnoofuse(err)
	}
	attrof(info, &out.Attr)
	return 0
}

func (t *fusenode) Setattr(ctx context.Context, f gofs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	var (
		name = t.name()
		info fs.FileInfo
		err  error
	)

	if mode, ok := in.GetMode(); ok {
		if err = t.fsys.Chmod(name, fs.FileMode(mode&07777)); err != nil {
			return errnoofuse(err)
		}
	}

	uid, uok := in.GetUID()
	gid, gok := in.GetGID()
	if uok || gok {
		if info, err = t.fsys.Lstat(name); err != nil {
			return errnoofuse(err)
		}
		owner := info.Sys().(*nfsfile.FileInfo)
		if !uok {
			uid = owner.UID
		}
		if !gok {
			gid = owner.GID
		}
		if err = t.fsys.Lchown(name, int(uid), int(gid)); err != nil {
			return errnoofuse(err)
		}
	}

	if size, ok := in.GetSize(); ok {
		if err = t.truncate(f, name, int64(size)); err != nil {
			return errnoofuse(err)
		}
	}

	atime, aok := in.GetATime()
	mtime, mok := in.GetMTime()
	if aok || mok {
		if info, err = t.fsys.Lstat(name); err != nil {
			return errnoofuse(err)
		}
		if !aok {
			atime = info.ModTime()
		}
		if !mok {
			mtime = info.ModTime()
		}
		if err = t.fsys.Chtimes(name, atime, mtime); err != nil {
			return errnoofuse(err)
		}
	}

	return t.Getattr(ctx, f, out)
}

// truncate resizes name through the handle open on it, else through a handle of its own.
func (t *fusenode) truncate(f gofs.FileHandle, name string, size int64) (err error) {
	var (
		handle billy.File
	)

	if open, ok := f.(*fusefile); ok {
		return open.f.Truncate(size)
	}

	if handle, err = t.fsys.OpenFile(name, os.O_WRONLY, 0); err != nil {
		return err
	}
	defer handle.Close()
	return handle.Truncate(size)
}

func (t *fusenode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*gofs.Inode, syscall.Errno) {
	return t.entry(ctx, t.child(name), out)
}

func (t *fusenode) Readdir(ctx context.Context) (gofs.DirStream, syscall.Errno) {
	var (
		attr fuse.Attr
	)

	infos, err := t.fsys.ReadDir(t.name())
	if err != nil {
		return nil, errnoofuse(err)
	}

	entries := make([]fuse.DirEntry, 0, len(infos))
	for _, info := range infos {
		attrof(info, &attr)
		entries = append(entries, fuse.DirEntry{Name: info.Name(), Mode: attr.Mode, Ino: attr.Ino})
	}
	return gofs.NewListDirStream(entries), 0
}

func (t *fusenode) Open(ctx context.Context, flags uint32) (gofs.FileHandle, uint32, syscall.Errno) {
	f, err := t.fsys.OpenFile(t.name(), int(flags)&(os.O_RDONLY|os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_TRUNC), 0)
	if err != nil {
		return nil, 0, errnoofuse(err)
	}
	return &fusefile{f: f}, fuse.FOPEN_KEEP_CACHE, 0
}

func (t *fusenode) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*gofs.Inode, gofs.FileHandle, uint32, syscall.Errno) {
	var (
		created = t.child(name)
		node    *gofs.Inode
	)

	f, err := t.fsys.OpenFile(created, int(flags)&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_TRUNC|os.O_EXCL)|os.O_CREATE, fs.FileMode(mode&07777))
	if err != nil {
		return nil, nil, 0, errnoofuse(err)
	}
	errno := t.owned(ctx, created)
	if errno == 0 {
		node, errno = t.entry(ctx, created, out)
	}
	if errno != 0 {
		errorsx.Log(f.Close())
		return nil, nil, 0, errno
	}
	return node, &fusefile{f: f}, 0, 0
}

func (t *fusenode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*gofs.Inode, syscall.Errno) {
	var (
		created = t.child(name)
	)

	if _, err := t.fsys.Lstat(created); err == nil {
		return nil, syscall.EEXIST
	}
	if err := t.fsys.MkdirAll(created, fs.FileMode(mode&07777)); err != nil {
		return nil, errnoofuse(err)
	}
	if errno := t.owned(ctx, created); errno != 0 {
		return nil, errno
	}
	return t.entry(ctx, created, out)
}

func (t *fusenode) Unlink(ctx context.Context, name string) syscall.Errno {
	return errnoofuse(t.fsys.Remove(t.child(name)))
}

func (t *fusenode) Rmdir(ctx context.Context, name string) syscall.Errno {
	return errnoofuse(t.fsys.Remove(t.child(name)))
}

// Rename refuses renameat2's flags, which callers fall back from to a plain rename.
func (t *fusenode) Rename(ctx context.Context, name string, parent gofs.InodeEmbedder, newname string, flags uint32) syscall.Errno {
	if flags != 0 {
		return syscall.EINVAL
	}
	return errnoofuse(t.fsys.Rename(t.child(name), parent.(*fusenode).child(newname)))
}

func (t *fusenode) Symlink(ctx context.Context, target string, name string, out *fuse.EntryOut) (*gofs.Inode, syscall.Errno) {
	var (
		created = t.child(name)
	)

	if err := t.fsys.Symlink(target, created); err != nil {
		return nil, errnoofuse(err)
	}
	if errno := t.owned(ctx, created); errno != 0 {
		return nil, errno
	}
	return t.entry(ctx, created, out)
}

func (t *fusenode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	target, err := t.fsys.Readlink(t.name())
	if err != nil {
		return nil, errnoofuse(err)
	}
	return []byte(target), 0
}

// Fsync succeeds without syncing, as fusefile's does, for a directory or a file with no handle.
func (t *fusenode) Fsync(ctx context.Context, f gofs.FileHandle, flags uint32) syscall.Errno {
	return 0
}

// Statfs reports the filesystem holding the upper, which the root's writes fill.
func (t *fusenode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	var (
		st syscall.Statfs_t
	)

	if err := syscall.Statfs(t.fsys.sys.upper.dir.Name(), &st); err != nil {
		return errnoofuse(err)
	}
	out.FromStatfsT(&st)
	return 0
}

// fusefile is an open file. Reads and writes arrive concurrently with their offsets, and a file of
// an archive reads, as every handle writes, where it seeks.
type fusefile struct {
	m sync.Mutex
	f billy.File
}

var (
	_ gofs.FileReader   = (*fusefile)(nil)
	_ gofs.FileWriter   = (*fusefile)(nil)
	_ gofs.FileFsyncer  = (*fusefile)(nil)
	_ gofs.FileReleaser = (*fusefile)(nil)
)

func (t *fusefile) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	t.m.Lock()
	defer t.m.Unlock()

	n, err := t.f.ReadAt(dest, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, errnoofuse(err)
	}
	return fuse.ReadResultData(dest[:n]), 0
}

func (t *fusefile) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	t.m.Lock()
	defer t.m.Unlock()

	if _, err := t.f.Seek(off, io.SeekStart); err != nil {
		return 0, errnoofuse(err)
	}
	n, err := t.f.Write(data)
	return uint32(n), errnoofuse(err)
}

// Fsync succeeds without syncing: the upper does not outlive the machine, so nothing waits on its
// writes reaching the disk.
func (t *fusefile) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	return 0
}

func (t *fusefile) Release(ctx context.Context) syscall.Errno {
	return errnoofuse(t.f.Close())
}

// attrof describes info, as nfsfs reports it, in out.
func attrof(info fs.FileInfo, out *fuse.Attr) {
	var (
		sys   = info.Sys().(*nfsfile.FileInfo)
		mtime = info.ModTime()
	)

	out.Ino = sys.Fileid
	out.Size = uint64(info.Size())
	out.Blocks = (out.Size + 511) / 512
	out.Mode = unixmode(info.Mode())
	out.Nlink = sys.Nlink
	out.Uid, out.Gid = sys.UID, sys.GID
	out.SetTimes(&mtime, &mtime, &mtime)
}

// unixmode is mode as a stat's st_mode.
func unixmode(mode fs.FileMode) uint32 {
	var (
		converted = uint32(mode.Perm())
	)

	switch {
	case mode.IsDir():
		converted |= syscall.S_IFDIR
	case mode&fs.ModeSymlink != 0:
		converted |= syscall.S_IFLNK
	case mode&fs.ModeNamedPipe != 0:
		converted |= syscall.S_IFIFO
	case mode&fs.ModeSocket != 0:
		converted |= syscall.S_IFSOCK
	case mode&fs.ModeCharDevice != 0:
		converted |= syscall.S_IFCHR
	case mode&fs.ModeDevice != 0:
		converted |= syscall.S_IFBLK
	default:
		converted |= syscall.S_IFREG
	}

	if mode&fs.ModeSetuid != 0 {
		converted |= syscall.S_ISUID
	}
	if mode&fs.ModeSetgid != 0 {
		converted |= syscall.S_ISGID
	}
	if mode&fs.ModeSticky != 0 {
		converted |= syscall.S_ISVTX
	}
	return converted
}

// errnoofuse maps an error, however it is wrapped, to the errno a FUSE request answers with.
func errnoofuse(err error) syscall.Errno {
	var (
		errno syscall.Errno
	)

	switch {
	case err == nil:
		return 0
	case errors.As(err, &errno):
		return errno
	case errorsx.Is(err, fs.ErrNotExist):
		return syscall.ENOENT
	case errorsx.Is(err, fs.ErrExist):
		return syscall.EEXIST
	case errorsx.Is(err, fs.ErrInvalid):
		return syscall.EINVAL
	case errorsx.Is(err, fs.ErrPermission):
		return syscall.EPERM
	default:
		return syscall.EIO
	}
}
