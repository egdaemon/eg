package egiso

import (
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
)

// Tree is the filesystem an environment sees: archives and host directories mounted at paths, each
// exposing the owners of its files as virtual users through its own Owners. A path belongs to the
// mount at its longest prefix, and symlinks resolve within that mount. Directories leading to a
// mount point that no mount provides appear as directories owned by Privileged. Mount a host
// directory with Bind, which keeps its symlinks inside it and lets the environment write to it.
type Tree struct {
	mounts []mount
}

type mount struct {
	at     string
	src    fs.FS
	root   *os.Root // the host directory a Bind mounts, which writes go to
	users  Owners
	groups Owners
}

// Mount mounts src at the path at, "." for the root, exposing its files' uids through users and
// their gids through groups. A later mount at the same path replaces the earlier one.
func (t Tree) Mount(at string, src fs.FS, users Owners, groups Owners) Tree {
	return t.attach(mount{at: at, src: src, users: users, groups: groups})
}

// Bind mounts the host directory dir at the path at, like Mount, and the environment's writes
// within it go to dir as the host user, so the host user can change and delete what it writes.
func (t Tree) Bind(at string, dir *os.Root, users Owners, groups Owners) Tree {
	return t.attach(mount{at: at, src: dir.FS(), root: dir, users: users, groups: groups})
}

func (t Tree) attach(m mount) Tree {
	t.mounts = slices.DeleteFunc(slices.Clone(t.mounts), func(o mount) bool { return o.at == m.at })
	t.mounts = append(t.mounts, m)
	return t
}

// Open opens the named file. A directory lists the mounts within it alongside its own entries.
func (t Tree) Open(name string) (fs.File, error) {
	st, err := t.Stat(name)
	if err != nil {
		return nil, err
	}

	if st.IsDir() {
		return &dir{tree: t, name: name, info: st}, nil
	}

	m, rel, _ := t.mount(name)
	return m.src.Open(rel)
}

// Stat describes the named file, following a final symlink within its mount.
func (t Tree) Stat(name string) (fs.FileInfo, error) {
	return t.stat("stat", name, fs.Stat)
}

// Lstat describes the named file without following a final symlink.
func (t Tree) Lstat(name string) (fs.FileInfo, error) {
	return t.stat("lstat", name, fs.Lstat)
}

// ReadLink returns the target of the named symlink.
func (t Tree) ReadLink(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}

	m, rel, ok := t.mount(name)
	if !ok {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrNotExist}
	}
	return fs.ReadLink(m.src, rel)
}

func (t Tree) stat(op string, name string, stat func(fs.FS, string) (fs.FileInfo, error)) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}

	if m, rel, ok := t.mount(name); ok {
		st, err := stat(m.src, rel)
		if err == nil {
			return info{FileInfo: st, name: path.Base(name)}, nil
		}
		if !errorsx.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}

	if t.leads(name) {
		return synthetic(path.Base(name)), nil
	}

	return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
}

// ReadDir lists the named directory, sorted by name: its own entries, with the mount points within
// it in place of whatever they cover.
func (t Tree) ReadDir(name string) ([]fs.DirEntry, error) {
	var (
		entries = map[string]fs.DirEntry{}
	)

	st, err := t.Stat(name)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: syscall.ENOTDIR}
	}

	if m, rel, ok := t.mount(name); ok {
		listed, err := fs.ReadDir(m.src, rel)
		if err != nil && !errorsx.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		for _, e := range listed {
			entries[e.Name()] = e
		}
	}

	for _, m := range t.mounts {
		child, ok := within(name, m.at)
		if !ok {
			continue
		}
		if _, listed := entries[child]; listed && path.Join(name, child) != m.at {
			continue
		}
		childinfo, err := t.Stat(path.Join(name, child))
		if err != nil {
			return nil, err
		}
		entries[child] = fs.FileInfoToDirEntry(childinfo)
	}

	listing := slices.Collect(maps.Values(entries))
	slices.SortFunc(listing, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return listing, nil
}

// Owner returns the virtual user and group the named file, not following a final symlink, is
// exposed as owned by.
func (t Tree) Owner(name string) (user string, group string, err error) {
	st, err := t.Lstat(name)
	if err != nil {
		return "", "", err
	}

	if _, ok := st.Sys().(syntheticsys); ok {
		return Privileged, Privileged, nil
	}

	m, _, _ := t.mount(name)
	id, ok, err := source(m.src, st)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", errorsx.Errorf("%s has no owner in its source", name)
	}

	return m.users.Lookup(id.uid, id.user), m.groups.Lookup(id.gid, id.group), nil
}

// mount returns the mount name belongs to, the one at its longest prefix, and name within it.
func (t Tree) mount(name string) (found mount, rel string, ok bool) {
	for _, m := range t.mounts {
		r, inside := relative(m.at, name)
		if inside && (!ok || len(m.at) > len(found.at) || found.at == ".") {
			found, rel, ok = m, r, true
		}
	}
	return found, rel, ok
}

// binds reports whether the tree holds a Bind mount.
func (t Tree) binds() bool {
	return slices.ContainsFunc(t.mounts, func(m mount) bool { return m.root != nil })
}

// leads reports whether name is a directory on the way to a mount point.
func (t Tree) leads(name string) bool {
	return slices.ContainsFunc(t.mounts, func(m mount) bool {
		_, ok := within(name, m.at)
		return ok
	})
}

// relative returns name within the mount at at.
func relative(at string, name string) (string, bool) {
	switch {
	case at == ".":
		return name, true
	case name == at:
		return ".", true
	case strings.HasPrefix(name, at+"/"):
		return name[len(at)+1:], true
	default:
		return "", false
	}
}

// within returns the child of the directory name that leads to the mount point at.
func within(name string, at string) (string, bool) {
	if at == "." || at == name {
		return "", false
	}
	rest, ok := relative(name, at)
	if !ok {
		return "", false
	}
	child, _, _ := strings.Cut(rest, "/")
	return child, true
}

// syntheticsys marks a directory no mount provides.
type syntheticsys struct{}

type syntheticinfo string

func synthetic(name string) fs.FileInfo    { return syntheticinfo(name) }
func (t syntheticinfo) Name() string       { return string(t) }
func (t syntheticinfo) Size() int64        { return 0 }
func (t syntheticinfo) Mode() fs.FileMode  { return fs.ModeDir | 0755 }
func (t syntheticinfo) ModTime() time.Time { return time.Time{} }
func (t syntheticinfo) IsDir() bool        { return true }
func (t syntheticinfo) Sys() any           { return syntheticsys{} }

// dir is a directory of the tree, listed through the tree so its mount points appear in it.
type dir struct {
	tree    Tree
	name    string
	info    fs.FileInfo
	pending []fs.DirEntry
	listed  bool
}

func (t *dir) Stat() (fs.FileInfo, error) { return t.info, nil }

func (t *dir) Close() error { return nil }

func (t *dir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: t.name, Err: syscall.EISDIR}
}

// ReadDir lists the directory n entries at a time, or all of the rest when n is not positive.
func (t *dir) ReadDir(n int) (_ []fs.DirEntry, err error) {
	if !t.listed {
		if t.pending, err = t.tree.ReadDir(t.name); err != nil {
			return nil, err
		}
		t.listed = true
	}

	if n <= 0 {
		entries := t.pending
		t.pending = nil
		return entries, nil
	}

	if len(t.pending) == 0 {
		return nil, io.EOF
	}

	entries := t.pending[:min(n, len(t.pending))]
	t.pending = t.pending[len(entries):]
	return entries, nil
}
