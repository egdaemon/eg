package egvm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/moby/sys/mountinfo"
	"github.com/opencontainers/go-digest"
	"golang.org/x/sys/unix"
)

// rootsocket is the socket, in the directory the host shares at sshdir, on which the host serves the
// roots of the machines booted within a machine. A root served by a machine over its own root,
// itself served over FUSE, costs a round trip to the host for every directory it walks, so the host
// serves it from the egiso and an upper of its own instead.
const rootsocket = "egiso"

// the device numbers of /dev/fuse.
const (
	fusemajor = 10
	fuseminor = 229
)

// rootrequest is what a machine asks of the host on rootsocket: first to serve the root of the egiso
// Egiso names, a path within a directory the host mounts in the machine or an https URL, on the
// /dev/fuse descriptor sent with it, then to glob Glob or read Read in that root.
type rootrequest struct {
	Egiso   string   `json:"egiso,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
	Glob    string   `json:"glob,omitempty"`
	Read    string   `json:"read,omitempty"`
}

// rootreply answers a rootrequest.
type rootreply struct {
	Error string   `json:"error,omitempty"`
	Names []string `json:"names,omitempty"`
	Data  []byte   `json:"data,omitempty"`
}

// listen serves, on the socket at path, the roots the machine m describes asks for, until it closes.
func (t *VM) listen(path string, m Machine) (err error) {
	var (
		li *net.UnixListener
	)

	if li, err = net.ListenUnix("unixpacket", &net.UnixAddr{Name: path, Net: "unixpacket"}); err != nil {
		return errorsx.Wrap(err, "unable to serve the roots of the machine's machines")
	}
	t.closers = append(t.closers, li)

	go t.accept(li, m)
	return nil
}

// accept serves each machine that connects to li until li closes.
func (t *VM) accept(li *net.UnixListener, m Machine) {
	var (
		conn *net.UnixConn
		err  error
	)

	for {
		if conn, err = li.AcceptUnix(); err != nil {
			return
		}
		go t.roots(conn, m)
	}
}

// roots serves the root the machine asks for on conn, then answers its requests to read that root
// until it hangs up.
func (t *VM) roots(conn *net.UnixConn, m Machine) {
	var (
		req   rootrequest
		dev   *os.File
		fsys  *egiso.SysFS
		reply rootreply
		err   error
	)
	defer conn.Close()

	if req, dev, err = receive(conn); err != nil {
		errorsx.Log(errorsx.Wrap(err, "unable to receive the root a machine asked for"))
		return
	}
	if dev == nil {
		errorsx.Log(send(conn, rootreply{Error: "the machine sent no descriptor to serve its root on"}, nil))
		return
	}
	if fsys, err = t.nested(m, req.Egiso, req.Aliases, dev); err != nil {
		errorsx.Log(dev.Close())
		errorsx.Log(send(conn, rootreply{Error: err.Error()}, nil))
		return
	}
	if err = send(conn, rootreply{}, nil); err != nil {
		errorsx.Log(err)
		return
	}

	for {
		if req, dev, err = receive(conn); err != nil {
			return
		}
		if dev != nil {
			errorsx.Log(dev.Close())
		}
		reply = rootreply{}
		switch {
		case req.Glob != "":
			reply.Names, err = fs.Glob(fsys.Upper(), req.Glob)
		case req.Read != "":
			reply.Data, err = fs.ReadFile(fsys.Upper(), req.Read)
		default:
			err = errorsx.String("the machine asked for nothing")
		}
		if err != nil {
			reply = rootreply{Error: err.Error()}
		}
		if err = send(conn, reply, nil); err != nil {
			return
		}
	}
}

// nested serves dev, a /dev/fuse descriptor a machine booted within this one mounted, from the egiso
// uri names under an upper of its own, until the mount goes away. A path is opened within the
// directory the machine mounts it from, as named or by one of its aliases, so the machine reaches
// nothing it does not already see.
func (t *VM) nested(m Machine, uri string, aliases []string, dev *os.File) (_ *egiso.SysFS, err error) {
	var (
		st       unix.Stat_t
		archive  *egiso.Archive
		f        *os.File
		toc      digest.Digest
		cache    = userx.DefaultCacheDirectory("egvm")
		dir      string
		upper    *os.Root
		fsys     *egiso.SysFS
		users    []egiso.Account
		groups   []egiso.Account
		location string
		fragment string
	)

	if err = unix.Fstat(int(dev.Fd()), &st); err != nil {
		return nil, err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFCHR || unix.Major(uint64(st.Rdev)) != fusemajor || unix.Minor(uint64(st.Rdev)) != fuseminor {
		return nil, errorsx.String("the machine sent a descriptor that is not /dev/fuse")
	}

	location, fragment, _ = strings.Cut(uri, "#")
	switch {
	case strings.HasPrefix(location, "https://"):
		archive, err = egiso.OpenURI(context.Background(), m.client, uri)
	default:
		if toc, err = digest.Parse(fragment); err != nil {
			return nil, errorsx.Wrapf(err, "%s needs the digest of its table of contents as its fragment", location)
		}
		if f, err = m.open(location, aliases...); err != nil {
			return nil, err
		}
		archive, err = egiso.OpenFile(f, toc)
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			errorsx.Log(archive.Close())
		}
	}()

	if err = os.MkdirAll(cache, 0700); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machines' cache")
	}
	if dir, err = os.MkdirTemp(cache, "machine.*"); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine's upper")
	}
	if upper, err = os.OpenRoot(dir); err != nil {
		return nil, errorsx.Compact(errorsx.Wrap(err, "unable to open the machine's upper"), os.RemoveAll(dir))
	}
	if fsys, users, groups, err = egiso.NewRoot(archive, upper); err != nil {
		return nil, errorsx.Compact(err, upper.Close(), os.RemoveAll(dir))
	}

	t.served.Add(1)
	go func() {
		defer t.served.Done()
		errorsx.Log(egiso.ServeFUSE(dev, fsys, egiso.NFSOptionAccounts(users, groups)))
		errorsx.Log(errorsx.Compact(dev.Close(), archive.Close(), upper.Close(), os.RemoveAll(dir)))
	}()
	return fsys, nil
}

// open opens the file at path in the machine, or at the first of aliases, the other paths the machine
// reaches it at, within a directory the host mounts in it, from that directory, without leaving it.
func (t Machine) open(path string, aliases ...string) (_ *os.File, err error) {
	var (
		found  mount
		rel    string
		within string
		root   *os.Root
	)

	for _, p := range slices.Concat([]string{path}, aliases) {
		if !filepath.IsAbs(p) {
			return nil, errorsx.Errorf("%s is not an absolute path", p)
		}
		for _, mnt := range t.mounts {
			if within, err = filepath.Rel(mnt.path, p); err != nil || within == ".." || strings.HasPrefix(within, "../") {
				continue
			}
			if len(mnt.path) > len(found.path) {
				found, rel = mnt, within
			}
		}
		if found.dir != "" {
			break
		}
	}
	if found.dir == "" {
		return nil, errorsx.Errorf("%s is not within a directory the host mounts in the machine", path)
	}

	if root, err = os.OpenRoot(found.dir); err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(rel)
}

// host is the host of the machine Boot runs in, serving the roots of machines booted within it.
type host struct {
	m    sync.Mutex
	conn *net.UnixConn
}

var (
	_ fs.GlobFS     = (*host)(nil)
	_ fs.ReadFileFS = (*host)(nil)
)

// dialhost connects to the host serving the roots of machines booted within the machine this runs
// in, or returns nil when it runs in none.
func dialhost() (_ *host, err error) {
	var (
		conn *net.UnixConn
	)

	if conn, err = net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: filepath.Join(sshdir, rootsocket), Net: "unixpacket"}); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, errorsx.Wrap(err, "unable to reach the host serving machines' roots")
	}
	return &host{conn: conn}, nil
}

// serve has the host serve the root of the egiso uri names on dev, the descriptor of its mount.
func (t *host) serve(dev *os.File, uri string) (err error) {
	var (
		location string
		fragment string
		others   []string
		reply    rootreply
	)

	// the host finds the egiso by the path the machine sees it at, not by the links leading to it.
	if location, fragment, _ = strings.Cut(uri, "#"); !strings.HasPrefix(location, "https://") {
		if location, err = filepath.EvalSymlinks(location); err != nil {
			return err
		}
		if others, err = aliases(location); err != nil {
			return err
		}
		uri = location + "#" + fragment
	}

	if reply, err = t.ask(rootrequest{Egiso: uri, Aliases: others}, dev); err != nil {
		return err
	}
	if reply.Error != "" {
		return errorsx.Errorf("the host did not serve the root: %s", reply.Error)
	}
	return nil
}

// aliases returns the other paths path is reachable at in this mount namespace, through the other
// mounts of the filesystem holding it: a machine mounts its directories under /eg.mnt, where the
// host knows them, and binds them again under /workload.
func aliases(path string) (_ []string, err error) {
	var (
		mounts  []*mountinfo.Info
		holding *mountinfo.Info
		within  string
		rel     string
		found   []string
	)

	if mounts, err = mountinfo.GetMounts(nil); err != nil {
		return nil, err
	}

	for _, m := range mounts {
		if within, err = filepath.Rel(m.Mountpoint, path); err != nil || within == ".." || strings.HasPrefix(within, "../") {
			continue
		}
		if holding == nil || len(m.Mountpoint) > len(holding.Mountpoint) {
			holding, rel = m, within
		}
	}
	if holding == nil {
		return nil, nil
	}

	// the file's path within its filesystem, which each mount of that filesystem shows from its root.
	rel = filepath.Join(holding.Root, rel)
	for _, m := range mounts {
		if m == holding || m.Major != holding.Major || m.Minor != holding.Minor {
			continue
		}
		if within, err = filepath.Rel(m.Root, rel); err != nil || within == ".." || strings.HasPrefix(within, "../") {
			continue
		}
		found = append(found, filepath.Join(m.Mountpoint, within))
	}
	return found, nil
}

// Open is unsupported: the host globs and reads the root, it does not open it.
func (t *host) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: syscall.ENOTSUP}
}

// Glob returns the names in the root the host serves matching pattern.
func (t *host) Glob(pattern string) (_ []string, err error) {
	var (
		reply rootreply
	)

	if reply, err = t.ask(rootrequest{Glob: pattern}, nil); err != nil {
		return nil, err
	}
	if reply.Error != "" {
		return nil, errors.New(reply.Error)
	}
	return reply.Names, nil
}

// ReadFile returns the contents of the named file in the root the host serves.
func (t *host) ReadFile(name string) (_ []byte, err error) {
	var (
		reply rootreply
	)

	if reply, err = t.ask(rootrequest{Read: name}, nil); err != nil {
		return nil, err
	}
	if reply.Error != "" {
		return nil, &fs.PathError{Op: "read", Path: name, Err: errors.New(reply.Error)}
	}
	return reply.Data, nil
}

// ask sends req, with dev when it is not nil, and returns the host's reply.
func (t *host) ask(req rootrequest, dev *os.File) (reply rootreply, err error) {
	var (
		b []byte
	)

	t.m.Lock()
	defer t.m.Unlock()

	if err = send(t.conn, req, dev); err != nil {
		return reply, err
	}
	if b, _, err = read(t.conn); err != nil {
		return reply, err
	}
	return reply, json.Unmarshal(b, &reply)
}

// Close hangs up on the host, which keeps serving the root until its mount goes away.
func (t *host) Close() error {
	return t.conn.Close()
}

// send writes v as one message on conn, with dev when it is not nil.
func send(conn *net.UnixConn, v any, dev *os.File) (err error) {
	var (
		b   []byte
		oob []byte
	)

	if b, err = json.Marshal(v); err != nil {
		return err
	}
	if dev != nil {
		oob = unix.UnixRights(int(dev.Fd()))
	}
	_, _, err = conn.WriteMsgUnix(b, oob, nil)
	return err
}

// receive reads one rootrequest from conn, and the descriptor sent with it, if any.
func receive(conn *net.UnixConn) (req rootrequest, dev *os.File, err error) {
	var (
		b []byte
	)

	if b, dev, err = read(conn); err != nil {
		return req, nil, err
	}
	if err = json.Unmarshal(b, &req); err != nil {
		if dev != nil {
			errorsx.Log(dev.Close())
		}
		return req, nil, err
	}
	return req, dev, nil
}

// read reads one message from conn, and the descriptor sent with it, if any.
func read(conn *net.UnixConn) (_ []byte, dev *os.File, err error) {
	var (
		b    = make([]byte, 1<<20)
		oob  = make([]byte, unix.CmsgSpace(4))
		n    int
		oobn int
		msgs []unix.SocketControlMessage
		fds  []int
	)

	if n, oobn, _, _, err = conn.ReadMsgUnix(b, oob); err != nil {
		return nil, nil, err
	}
	if n == 0 && oobn == 0 {
		return nil, nil, io.EOF
	}
	if msgs, err = unix.ParseSocketControlMessage(oob[:oobn]); err != nil {
		return nil, nil, err
	}
	for _, msg := range msgs {
		if fds, err = unix.ParseUnixRights(&msg); err != nil {
			return nil, nil, err
		}
		for _, fd := range fds {
			unix.CloseOnExec(fd)
			if dev != nil {
				errorsx.Log(unix.Close(fd))
				continue
			}
			dev = os.NewFile(uintptr(fd), "fuse")
		}
	}
	return b[:n], dev, nil
}
