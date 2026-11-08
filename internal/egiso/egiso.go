// Package egiso reads egiso images: eStargz archives, read lazily through their table of contents.
// An archive is pinned to the digest of its table of contents, and every chunk of a file is
// checked against the digest the table records for it before any of it is returned. The archive
// is a read-only fs.FS whose symlinks resolve within the archive; a file's FileInfo.Sys is its
// *estargz.TOCEntry, which holds the owner (UID, GID, Uname, Gname) the image records.
//
// A Tree mounts archives and host directories at paths, and exposes the owners of each mount's files
// as the environment's virtual users through that mount's own Owners.
package egiso

import (
	"context"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"syscall"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/opencontainers/go-digest"
)

// symlinks followed while resolving one name before giving up, like linux's MAXSYMLINKS.
const maxsymlinks = 40

// Archive is an egiso opened for reading.
type Archive struct {
	r      *estargz.Reader
	v      estargz.TOCEntryVerifier
	closer io.Closer
	named  func() (names, error)
}

// Open opens the egiso at location, a local path or an https URL read with range requests, pinned
// to the digest of its table of contents.
func Open(ctx context.Context, location string, toc digest.Digest) (_ *Archive, err error) {
	var (
		u  *url.URL
		f  *os.File
		st os.FileInfo
		sr *io.SectionReader
		a  *Archive
	)

	if u, err = url.Parse(location); err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "https":
		if sr, err = Remote(ctx, nil, location); err != nil {
			return nil, err
		}
		return New(sr, toc)
	case "":
	default:
		return nil, errorsx.Errorf("unsupported egiso location %q, use a path or an https URL", location)
	}

	if f, err = os.Open(location); err != nil {
		return nil, err
	}
	if st, err = f.Stat(); err != nil {
		return nil, errorsx.Compact(err, f.Close())
	}

	if a, err = New(io.NewSectionReader(f, 0, st.Size()), toc); err != nil {
		return nil, errorsx.Compact(err, f.Close())
	}
	a.closer = f
	return a, nil
}

// New opens the egiso in sr, pinned to the digest of its table of contents.
func New(sr *io.SectionReader, toc digest.Digest) (_ *Archive, err error) {
	var (
		r *estargz.Reader
		v estargz.TOCEntryVerifier
	)

	if r, err = estargz.Open(sr); err != nil {
		return nil, errorsx.Wrap(err, "unable to open the egiso")
	}

	if v, err = r.VerifyTOC(toc); err != nil {
		return nil, errorsx.Wrap(err, "unable to verify the egiso")
	}

	a := &Archive{r: r, v: v}
	a.named = sync.OnceValues(func() (names, error) { return directory(a) })
	return a, nil
}

// Build converts the tar archive to an egiso written to w, returning the digest of its table of
// contents, which Open and New pin it to.
func Build(tar *io.SectionReader, w io.Writer, options ...estargz.Option) (_ digest.Digest, err error) {
	var (
		blob *estargz.Blob
	)

	if blob, err = estargz.Build(tar, options...); err != nil {
		return "", err
	}
	defer blob.Close()

	if _, err = io.Copy(w, blob); err != nil {
		return "", err
	}

	return blob.TOCDigest(), nil
}

// Close releases the file a local archive was opened from.
func (t *Archive) Close() error {
	if t.closer == nil {
		return nil
	}
	return t.closer.Close()
}

// Open opens the named file, following symlinks.
func (t *Archive) Open(name string) (fs.File, error) {
	var (
		e   *estargz.TOCEntry
		err error
	)

	if e, err = t.resolve("open", name, true); err != nil {
		return nil, err
	}

	f := &file{archive: t, name: name, e: e}
	if e.Type == "reg" {
		if f.data, err = t.r.OpenFile(e.Name); err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
	}

	return f, nil
}

// Stat describes the named file, following symlinks.
func (t *Archive) Stat(name string) (fs.FileInfo, error) {
	return t.stat("stat", name, true)
}

// Lstat describes the named file without following a final symlink.
func (t *Archive) Lstat(name string) (fs.FileInfo, error) {
	return t.stat("lstat", name, false)
}

// ReadLink returns the target of the named symlink.
func (t *Archive) ReadLink(name string) (string, error) {
	e, err := t.resolve("readlink", name, false)
	if err != nil {
		return "", err
	}
	if e.Type != "symlink" {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}
	return e.LinkName, nil
}

// ReadDir lists the named directory, sorted by name.
func (t *Archive) ReadDir(name string) ([]fs.DirEntry, error) {
	e, err := t.resolve("readdir", name, true)
	if err != nil {
		return nil, err
	}
	if e.Type != "dir" {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: syscall.ENOTDIR}
	}
	return t.entries(e), nil
}

func (t *Archive) stat(op string, name string, follow bool) (fs.FileInfo, error) {
	e, err := t.resolve(op, name, follow)
	if err != nil {
		return nil, err
	}
	return info{FileInfo: e.Stat(), name: path.Base(name)}, nil
}

// entries lists a directory's children. A hardlink is described by the file it links to, under its
// own name.
func (t *Archive) entries(dir *estargz.TOCEntry) (entries []fs.DirEntry) {
	dir.ForeachChild(func(base string, child *estargz.TOCEntry) bool {
		if landmark(child.Name) {
			return true
		}
		if resolved, ok := t.r.Lookup(child.Name); ok {
			child = resolved
		}
		entries = append(entries, fs.FileInfoToDirEntry(info{FileInfo: child.Stat(), name: base}))
		return true
	})

	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries
}

// resolve finds the entry for name, a slash separated path within the archive. Symlinks in the
// directories leading to it are followed, and so is a final symlink when follow is set. An
// absolute target resolves from the archive's root, and `..` stops at the root.
func (t *Archive) resolve(op string, name string, follow bool) (*estargz.TOCEntry, error) {
	var (
		dir   []string
		todo  []string
		hops  int
		e     *estargz.TOCEntry
		found bool
	)

	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	if name != "." {
		todo = strings.Split(name, "/")
	}

	for len(todo) > 0 {
		component := todo[0]
		todo = todo[1:]

		switch component {
		case "", ".":
			continue
		case "..":
			dir = dir[:max(len(dir)-1, 0)]
			continue
		}

		if e, found = t.r.Lookup(path.Join(append(dir, component)...)); !found || landmark(e.Name) {
			return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
		}

		if e.Type == "symlink" && (len(todo) > 0 || follow) {
			if hops++; hops > maxsymlinks {
				return nil, &fs.PathError{Op: op, Path: name, Err: syscall.ELOOP}
			}
			if path.IsAbs(e.LinkName) {
				dir = nil
			}
			todo = append(strings.Split(e.LinkName, "/"), todo...)
			continue
		}

		if e.Type != "dir" && len(todo) > 0 {
			return nil, &fs.PathError{Op: op, Path: name, Err: syscall.ENOTDIR}
		}

		dir = append(dir, component)
	}

	e, _ = t.r.Lookup(path.Join(dir...))
	return e, nil
}

// landmark reports whether the entry is one of the markers estargz adds at the archive's root for
// prefetching, which are not files of the image.
func landmark(name string) bool {
	return name == estargz.PrefetchLandmark || name == estargz.NoPrefetchLandmark
}

// info names an entry by the path it was opened with: a symlink or hardlink describes its target
// under its own name.
type info struct {
	fs.FileInfo
	name string
}

func (t info) Name() string {
	return t.name
}

type file struct {
	archive *Archive
	name    string
	e       *estargz.TOCEntry
	data    *io.SectionReader
	offset  int64
	// the verified chunk holding offset, and where in the file it starts.
	chunk   []byte
	chunkat int64
	// a directory's entries not yet returned by ReadDir.
	pending []fs.DirEntry
	listed  bool
}

func (t *file) Stat() (fs.FileInfo, error) {
	return info{FileInfo: t.e.Stat(), name: path.Base(t.name)}, nil
}

func (t *file) Close() error {
	return nil
}

// Read reads the file a verified chunk at a time.
func (t *file) Read(p []byte) (n int, err error) {
	if t.e.Type == "dir" {
		return 0, &fs.PathError{Op: "read", Path: t.name, Err: syscall.EISDIR}
	}
	if t.offset >= t.e.Size {
		return 0, io.EOF
	}

	if t.offset < t.chunkat || t.offset >= t.chunkat+int64(len(t.chunk)) {
		if err = t.load(t.offset); err != nil {
			return 0, &fs.PathError{Op: "read", Path: t.name, Err: err}
		}
	}

	n = copy(p, t.chunk[t.offset-t.chunkat:])
	t.offset += int64(n)
	return n, nil
}

// load reads the chunk holding off and checks it against the digest the table of contents records.
func (t *file) load(off int64) (err error) {
	var (
		v digest.Verifier
	)

	ce, ok := t.archive.r.ChunkEntryForOffset(t.e.Name, off)
	if !ok {
		return io.ErrUnexpectedEOF
	}

	if v, err = t.archive.v.Verifier(ce); err != nil {
		return err
	}

	chunk := make([]byte, ce.ChunkSize)
	if _, err = t.data.ReadAt(chunk, ce.ChunkOffset); err != nil && err != io.EOF {
		return err
	}

	if _, err = v.Write(chunk); err != nil {
		return err
	}
	if !v.Verified() {
		return errorsx.Errorf("chunk at %d does not match its digest", ce.ChunkOffset)
	}

	t.chunk, t.chunkat = chunk, ce.ChunkOffset
	return nil
}

// ReadDir lists a directory n entries at a time, or all of the rest when n is not positive.
func (t *file) ReadDir(n int) ([]fs.DirEntry, error) {
	if t.e.Type != "dir" {
		return nil, &fs.PathError{Op: "readdir", Path: t.name, Err: syscall.ENOTDIR}
	}

	if !t.listed {
		t.pending, t.listed = t.archive.entries(t.e), true
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
