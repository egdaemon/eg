package egiso

import (
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
)

// whiteouts follow the OCI image layer convention, so an upper can be stored as a layer.
const (
	whiteout = ".wh."
	opaque   = ".wh..wh..opq"
)

// attrs are the owner and mode the environment sees for an entry of the upper. The upper's files on
// disk all belong to the host user, with modes that keep them writable by it.
type attrs struct {
	user  string
	group string
	mode  fs.FileMode
}

// Upper is a workload's writable layer over a Tree, kept in the host directory dir. Changing an
// entry of the tree copies it up into dir first, and removing one records a whiteout in dir, so the
// tree itself never changes. Writes within a Bind mount go to its host directory instead. No
// method follows a final symlink: FUSE asks for each component of a path, and the kernel resolves
// the symlinks.
type Upper struct {
	tree  Tree
	dir   *os.Root
	m     sync.Mutex
	attrs map[string]attrs
}

// NewUpper layers the empty directory dir over tree.
func NewUpper(tree Tree, dir *os.Root) *Upper {
	return &Upper{tree: tree, dir: dir, attrs: map[string]attrs{}}
}

// Lstat describes the named file.
func (t *Upper) Lstat(name string) (fs.FileInfo, error) {
	t.m.Lock()
	defer t.m.Unlock()

	st, up, err := t.lstat("lstat", name)
	if err != nil || !up {
		return st, err
	}
	return attrinfo{FileInfo: st, mode: t.attrs[name].mode}, nil
}

// ReadLink returns the target of the named symlink.
func (t *Upper) ReadLink(name string) (string, error) {
	t.m.Lock()
	defer t.m.Unlock()

	_, up, err := t.lstat("readlink", name)
	if err != nil {
		return "", err
	}
	if up {
		return t.dir.Readlink(name)
	}
	return t.tree.ReadLink(name)
}

// Owner returns the virtual user and group the named file is exposed as owned by.
func (t *Upper) Owner(name string) (user string, group string, err error) {
	t.m.Lock()
	defer t.m.Unlock()

	_, up, err := t.lstat("owner", name)
	if err != nil {
		return "", "", err
	}
	if up {
		return t.attrs[name].user, t.attrs[name].group, nil
	}
	return t.tree.Owner(name)
}

// Open opens the named file for reading.
func (t *Upper) Open(name string) (fs.File, error) {
	t.m.Lock()
	defer t.m.Unlock()

	_, up, err := t.lstat("open", name)
	if err != nil {
		return nil, err
	}
	if up {
		return t.dir.Open(name)
	}
	return t.tree.Open(name)
}

// ReadDir lists the named directory, sorted by name.
func (t *Upper) ReadDir(name string) ([]fs.DirEntry, error) {
	t.m.Lock()
	defer t.m.Unlock()

	return t.readdir(name)
}

// OpenFile opens the named file for writing with the flags of os.OpenFile, copying it up first.
// A file it creates has mode perm and is owned by user and group.
func (t *Upper) OpenFile(name string, flag int, perm fs.FileMode, user string, group string) (*os.File, error) {
	dir, rel, flag, err := t.prepare(name, flag, perm, user, group)
	if err != nil {
		return nil, err
	}
	return dir.OpenFile(rel, flag, perm)
}

// Chtimes sets the named file's access and modification times, copying it up first.
func (t *Upper) Chtimes(name string, atime time.Time, mtime time.Time) error {
	if m, rel, ok := t.bound(name); ok {
		return m.root.Chtimes(rel, atime, mtime)
	}

	t.m.Lock()
	defer t.m.Unlock()

	if _, err := t.copied("chtimes", name); err != nil {
		return err
	}
	return t.dir.Chtimes(name, atime, mtime)
}

// prepare readies name to be opened with flag: within a Bind mount as it is, otherwise by copying
// it up or creating it in the upper. It returns the host directory to open it in, its name there,
// and the flags to open it with.
func (t *Upper) prepare(name string, flag int, perm fs.FileMode, user string, group string) (_ *os.Root, _ string, _ int, err error) {
	if m, rel, ok := t.bound(name); ok {
		return m.root, rel, flag, nil
	}

	t.m.Lock()
	defer t.m.Unlock()

	_, up, err := t.lstat("open", name)
	switch {
	case err == nil && flag&os.O_CREATE != 0 && flag&os.O_EXCL != 0:
		return nil, "", 0, &fs.PathError{Op: "open", Path: name, Err: fs.ErrExist}
	case err == nil && !up:
		err = t.copyup(name)
	case errorsx.Is(err, fs.ErrNotExist) && flag&os.O_CREATE != 0:
		err = t.create(name, perm.Perm(), user, group, func() error {
			return t.dir.WriteFile(name, nil, 0600)
		})
	}
	if err != nil {
		return nil, "", 0, err
	}

	return t.dir, name, flag &^ (os.O_CREATE | os.O_EXCL), nil
}

// Mkdir creates the named directory with mode perm, owned by user and group.
func (t *Upper) Mkdir(name string, perm fs.FileMode, user string, group string) error {
	if m, rel, ok := t.bound(name); ok {
		return m.root.Mkdir(rel, perm)
	}

	t.m.Lock()
	defer t.m.Unlock()

	if _, _, err := t.lstat("mkdir", name); err == nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}

	err := t.create(name, fs.ModeDir|perm.Perm(), user, group, func() error {
		return t.dir.Mkdir(name, 0700)
	})
	if err != nil {
		return err
	}

	// the tree's directory of the same name was removed, so none of its entries show through.
	if _, err = t.tree.Lstat(name); err == nil {
		return t.dir.WriteFile(path.Join(name, opaque), nil, 0600)
	}
	return nil
}

// Symlink creates the named symlink to target, owned by user and group.
func (t *Upper) Symlink(target string, name string, user string, group string) error {
	if m, rel, ok := t.bound(name); ok {
		return m.root.Symlink(target, rel)
	}

	t.m.Lock()
	defer t.m.Unlock()

	if _, _, err := t.lstat("symlink", name); err == nil {
		return &fs.PathError{Op: "symlink", Path: name, Err: fs.ErrExist}
	}

	return t.create(name, fs.ModeSymlink|0777, user, group, func() error {
		return t.dir.Symlink(target, name)
	})
}

// Remove removes the named file or empty directory.
func (t *Upper) Remove(name string) error {
	if m, rel, ok := t.bound(name); ok {
		if rel == "." {
			return &fs.PathError{Op: "remove", Path: name, Err: syscall.EBUSY}
		}
		return m.root.Remove(rel)
	}

	t.m.Lock()
	defer t.m.Unlock()

	st, up, err := t.lstat("remove", name)
	if err != nil {
		return err
	}
	if err = t.empty("remove", name, st); err != nil {
		return err
	}

	if up {
		if err = t.dir.RemoveAll(name); err != nil {
			return err
		}
		t.forget(name)
	}

	return t.whiteout(name)
}

// Rename renames oldname to newname, replacing a file or empty directory at newname. A directory
// that shows the tree's entries is copied up with everything in it first. Renaming into or out of
// a Bind mount fails with EXDEV.
func (t *Upper) Rename(oldname string, newname string) error {
	var (
		dst fs.FileInfo
		dup bool
	)

	mo, ro, bo := t.bound(oldname)
	mn, rn, bn := t.bound(newname)
	if bo && bn && mo.at == mn.at {
		return mo.root.Rename(ro, rn)
	}
	if bo || bn {
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: syscall.EXDEV}
	}

	t.m.Lock()
	defer t.m.Unlock()

	st, up, err := t.lstat("rename", oldname)
	if err != nil {
		return err
	}
	deep := st.IsDir() && t.merges(oldname, up)

	dst, dup, err = t.lstat("rename", newname)
	switch {
	case err == nil && dst.IsDir() && !st.IsDir():
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: syscall.EISDIR}
	case err == nil && !dst.IsDir() && st.IsDir():
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: syscall.ENOTDIR}
	case err == nil:
		if err = t.empty("rename", newname, dst); err != nil {
			return err
		}
	case !errorsx.Is(err, fs.ErrNotExist):
		return err
	}

	switch {
	case deep:
		if err = t.copytree(oldname); err != nil {
			return err
		}
	case !up:
		if err = t.copyup(oldname); err != nil {
			return err
		}
	}
	if err = t.parents(path.Dir(newname)); err != nil {
		return err
	}
	if dup {
		if err = t.dir.RemoveAll(newname); err != nil {
			return err
		}
		t.forget(newname)
	}
	if err = t.unwhiteout(newname); err != nil {
		return err
	}
	if err = t.dir.Rename(oldname, newname); err != nil {
		return err
	}

	for name, a := range maps.Clone(t.attrs) {
		if rel, ok := relative(oldname, name); ok {
			delete(t.attrs, name)
			t.attrs[path.Join(newname, rel)] = a
		}
	}

	// the tree's directory at newname was replaced, so none of its entries show through.
	if _, err = t.tree.Lstat(newname); err == nil && st.IsDir() {
		if err = t.dir.WriteFile(path.Join(newname, opaque), nil, 0600); err != nil {
			return err
		}
	}

	return t.whiteout(oldname)
}

// Chmod sets the named file's mode bits.
func (t *Upper) Chmod(name string, mode fs.FileMode) error {
	if m, rel, ok := t.bound(name); ok {
		return m.root.Chmod(rel, mode)
	}

	t.m.Lock()
	defer t.m.Unlock()

	a, err := t.copied("chmod", name)
	if err != nil {
		return err
	}
	a.mode = a.mode.Type() | mode&(fs.ModePerm|fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky)
	t.attrs[name] = a
	return nil
}

// Chown sets the virtual user and group that own the named file; an empty one is left unchanged.
// A Bind mount's files belong to the host user, so they can not change owner.
func (t *Upper) Chown(name string, user string, group string) error {
	if _, _, ok := t.bound(name); ok {
		return &fs.PathError{Op: "chown", Path: name, Err: syscall.EPERM}
	}

	t.m.Lock()
	defer t.m.Unlock()

	a, err := t.copied("chown", name)
	if err != nil {
		return err
	}
	a.user = langx.FirstNonZero(user, a.user)
	a.group = langx.FirstNonZero(group, a.group)
	t.attrs[name] = a
	return nil
}

// host returns the host directory holding name and its name there, when a Bind mount or the upper
// holds it.
func (t *Upper) host(name string) (*os.Root, string, bool) {
	if m, rel, ok := t.bound(name); ok {
		return m.root, rel, true
	}

	t.m.Lock()
	defer t.m.Unlock()

	if _, up, err := t.lstat("open", name); err == nil && up {
		return t.dir, name, true
	}
	return nil, "", false
}

// bound returns the Bind mount name is within.
func (t *Upper) bound(name string) (mount, string, bool) {
	m, rel, ok := t.tree.mount(name)
	return m, rel, ok && m.root != nil
}

// lstat describes name and reports whether the upper holds it.
func (t *Upper) lstat(op string, name string) (_ fs.FileInfo, up bool, err error) {
	if !fs.ValidPath(name) {
		return nil, false, &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}

	if _, _, ok := t.bound(name); ok || name == "." {
		st, err := t.tree.Lstat(name)
		return st, false, err
	}

	if strings.HasPrefix(path.Base(name), whiteout) {
		return nil, false, &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
	}

	if st, err := t.dir.Lstat(name); err == nil {
		return st, true, nil
	}

	if t.hidden(name) {
		return nil, false, &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
	}

	st, err := t.tree.Lstat(name)
	return st, false, err
}

// hidden reports whether the upper hides name in the tree: by a whiteout of it or of an ancestor,
// or by replacing an ancestor with a file or an opaque directory.
func (t *Upper) hidden(name string) bool {
	for p := name; p != "."; p = path.Dir(p) {
		if t.exists(whiteoutof(p)) {
			return true
		}
		if p == name {
			continue
		}
		if st, err := t.dir.Lstat(p); err == nil && (!st.IsDir() || t.exists(path.Join(p, opaque))) {
			return true
		}
	}
	return false
}

// merges reports whether the directory name shows the tree's entries.
func (t *Upper) merges(name string, up bool) bool {
	st, err := t.tree.Lstat(name)
	if err != nil || !st.IsDir() || t.hidden(name) {
		return false
	}
	return !up || !t.exists(path.Join(name, opaque))
}

func (t *Upper) exists(name string) bool {
	_, err := t.dir.Lstat(name)
	return err == nil
}

func (t *Upper) readdir(name string) ([]fs.DirEntry, error) {
	var (
		entries = map[string]fs.DirEntry{}
		upper   []fs.DirEntry
	)

	if _, _, ok := t.bound(name); ok {
		return t.tree.ReadDir(name)
	}

	st, up, err := t.lstat("readdir", name)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: syscall.ENOTDIR}
	}

	if t.merges(name, up) {
		lower, err := t.tree.ReadDir(name)
		if err != nil {
			return nil, err
		}
		for _, e := range lower {
			entries[e.Name()] = e
		}
	}

	if up {
		if upper, err = fs.ReadDir(t.dir.FS(), name); err != nil {
			return nil, err
		}
	}
	for _, e := range upper {
		switch hides, isWhiteout := strings.CutPrefix(e.Name(), whiteout); {
		case e.Name() == opaque:
		case isWhiteout:
			delete(entries, hides)
		default:
			entries[e.Name()] = attrentry{DirEntry: e, attrs: t.attrs[path.Join(name, e.Name())]}
		}
	}

	listing := slices.Collect(maps.Values(entries))
	slices.SortFunc(listing, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return listing, nil
}

// empty refuses to remove a directory that lists any entry.
func (t *Upper) empty(op string, name string, st fs.FileInfo) error {
	if !st.IsDir() {
		return nil
	}
	entries, err := t.readdir(name)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return &fs.PathError{Op: op, Path: name, Err: syscall.ENOTEMPTY}
	}
	return nil
}

// copyup copies the tree's entry name, without its contents if a directory, into the upper.
func (t *Upper) copyup(name string) (err error) {
	var (
		st          fs.FileInfo
		user, group string
		target      string
		src         fs.File
		dst         *os.File
	)

	if err = t.parents(path.Dir(name)); err != nil {
		return err
	}
	if st, err = t.tree.Lstat(name); err != nil {
		return err
	}
	if user, group, err = t.tree.Owner(name); err != nil {
		return err
	}

	switch {
	case st.IsDir():
		err = t.dir.Mkdir(name, 0700)
	case st.Mode()&fs.ModeSymlink != 0:
		if target, err = t.tree.ReadLink(name); err != nil {
			return err
		}
		err = t.dir.Symlink(target, name)
	default:
		if src, err = t.tree.Open(name); err != nil {
			return err
		}
		defer src.Close()
		if dst, err = t.dir.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600); err != nil {
			return err
		}
		defer dst.Close()
		_, err = io.Copy(dst, src)
	}
	if err != nil {
		return errorsx.Wrapf(err, "unable to copy up %s", name)
	}

	t.attrs[name] = attrs{user: user, group: group, mode: st.Mode()}
	return nil
}

// copytree copies the directory name and everything it lists into the upper, and makes it opaque so
// it no longer shows the tree's entries.
func (t *Upper) copytree(name string) error {
	entries, err := t.readdir(name)
	if err != nil {
		return err
	}
	if !t.exists(name) {
		if err = t.copyup(name); err != nil {
			return err
		}
	}

	for _, e := range entries {
		child := path.Join(name, e.Name())
		st, up, err := t.lstat("copyup", child)
		switch {
		case err != nil:
			return err
		case st.IsDir():
			err = t.copytree(child)
		case !up:
			err = t.copyup(child)
		}
		if err != nil {
			return err
		}
	}

	return t.dir.WriteFile(path.Join(name, opaque), nil, 0600)
}

// copied returns the attrs of name in the upper, copying it up first.
func (t *Upper) copied(op string, name string) (attrs, error) {
	_, up, err := t.lstat(op, name)
	if err != nil {
		return attrs{}, err
	}
	if !up {
		if err = t.copyup(name); err != nil {
			return attrs{}, err
		}
	}
	return t.attrs[name], nil
}

// parents copies up the directory name and its ancestors.
func (t *Upper) parents(name string) error {
	if name == "." || t.exists(name) {
		return nil
	}
	if err := t.parents(path.Dir(name)); err != nil {
		return err
	}

	st, _, err := t.lstat("copyup", name)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return &fs.PathError{Op: "copyup", Path: name, Err: syscall.ENOTDIR}
	}
	return t.copyup(name)
}

// create makes name in the upper with build, in place of a whiteout of it, and records its attrs.
func (t *Upper) create(name string, mode fs.FileMode, user string, group string, build func() error) error {
	if strings.HasPrefix(path.Base(name), whiteout) {
		return &fs.PathError{Op: "create", Path: name, Err: fs.ErrInvalid}
	}
	if err := t.parents(path.Dir(name)); err != nil {
		return err
	}
	if err := t.unwhiteout(name); err != nil {
		return err
	}
	if err := build(); err != nil {
		return err
	}

	t.attrs[name] = attrs{user: user, group: group, mode: mode}
	return nil
}

// whiteout hides the tree's entry name, if it would otherwise show.
func (t *Upper) whiteout(name string) error {
	if _, err := t.tree.Lstat(name); err != nil || t.hidden(name) {
		return nil
	}
	if err := t.parents(path.Dir(name)); err != nil {
		return err
	}
	return t.dir.WriteFile(whiteoutof(name), nil, 0600)
}

func (t *Upper) unwhiteout(name string) error {
	if err := t.dir.Remove(whiteoutof(name)); err != nil && !errorsx.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// forget drops the attrs of name and everything within it.
func (t *Upper) forget(name string) {
	maps.DeleteFunc(t.attrs, func(k string, _ attrs) bool {
		_, ok := relative(name, k)
		return ok
	})
}

func whiteoutof(name string) string {
	return path.Join(path.Dir(name), whiteout+path.Base(name))
}

// attrinfo describes an entry of the upper with the mode the environment sees.
type attrinfo struct {
	fs.FileInfo
	mode fs.FileMode
}

func (t attrinfo) Mode() fs.FileMode { return t.mode }

type attrentry struct {
	fs.DirEntry
	attrs attrs
}

func (t attrentry) Info() (fs.FileInfo, error) {
	st, err := t.DirEntry.Info()
	if err != nil {
		return nil, err
	}
	return attrinfo{FileInfo: st, mode: t.attrs.mode}, nil
}
