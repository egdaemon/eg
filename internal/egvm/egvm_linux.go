package egvm

import (
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/execproxy"
	subids "github.com/moby/sys/user"
	"go.podman.io/common/pkg/cgroups"
	"golang.org/x/sys/unix"
)

// dnsforward is the address pasta answers DNS on inside the machine, as it does for rootless podman.
const dnsforward = "169.254.1.1"

// VM is a running machine: the root's init in namespaces of its own, rooted at the egiso.
type VM struct {
	dir     string
	netns   string
	agent   string
	dev     *os.File
	init    *exec.Cmd
	stopped chan struct{}
	served  chan struct{}
}

// Boot starts the machine as an unprivileged user: its init runs as root of a user namespace mapping
// the user's subordinate ids, in mount, pid, uts, ipc, network and cgroup namespaces of its own,
// rooted at the egiso served over FUSE. pasta connects its network to the host's, as it does for
// rootless podman. Its init's output is copied to console.
func Boot(m Machine, console io.Writer) (_ *VM, err error) {
	var (
		t        = &VM{stopped: make(chan struct{})}
		root     string
		listener *net.UnixListener
		agent    *os.File
		mapped   [2]*os.File
		mounted  [2]int
		args     []string
		owned    bool
	)
	defer func() {
		if err != nil {
			errorsx.Log(t.Close())
		}
	}()

	// the root's systemd manages the cgroup it starts in, which the user must own for it to boot.
	if owned, err = cgroups.UserOwnsCurrentSystemdCgroup(); err != nil {
		return nil, errorsx.Wrap(err, "unable to find the cgroup the machine starts in")
	}
	if !owned {
		return nil, errorsx.New("machines need a cgroup the user owns: run in a systemd scope or service with Delegate=yes")
	}

	if t.dir, err = os.MkdirTemp("", "egvm.*"); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine's directory")
	}
	root, t.agent = filepath.Join(t.dir, "root"), filepath.Join(t.dir, "agent.sock")
	if err = os.Mkdir(root, 0755); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine's root")
	}

	// the network namespace file goes where rootless podman keeps its own, which pasta may read: the
	// user's runtime directory, which a service such as eg-daemon is not told of.
	t.netns = filepath.Join(envx.String(filepath.Join("/", "run", "user", strconv.Itoa(os.Getuid())), "XDG_RUNTIME_DIR"), "netns", filepath.Base(t.dir))
	if err = os.MkdirAll(filepath.Dir(t.netns), 0700); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the network namespace directory")
	}
	if err = os.WriteFile(t.netns, nil, 0600); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine's network namespace file")
	}

	if listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: t.agent, Net: "unix"}); err != nil {
		return nil, errorsx.Wrap(err, "unable to listen for the machine's agent")
	}
	// the agent listens on the socket after Boot closes its own copy.
	listener.SetUnlinkOnClose(false)
	defer listener.Close()
	if agent, err = listener.File(); err != nil {
		return nil, err
	}
	defer agent.Close()

	if mapped[0], mapped[1], err = os.Pipe(); err != nil {
		return nil, err
	}
	defer mapped[1].Close()
	if mounted, err = unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0); err != nil {
		return nil, errorsx.Compact(err, mapped[0].Close())
	}
	defer unix.Close(mounted[0])

	args = []string{stagemap, root, m.init, t.netns}
	for _, mnt := range m.mounts {
		args = append(args, mnt.dir, mnt.path, strconv.FormatBool(mnt.readonly))
	}

	t.init = &exec.Cmd{
		Path:        "/proc/self/exe",
		Args:        args,
		Stdout:      console,
		Stderr:      console,
		ExtraFiles:  []*os.File{mapped[0], os.NewFile(uintptr(mounted[1]), "mounted"), agent},
		SysProcAttr: &syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS, Pdeathsig: syscall.SIGKILL},
	}
	err = t.init.Start()
	mapped[0].Close()
	t.init.ExtraFiles[1].Close()
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to start the machine")
	}
	go func() {
		defer close(t.stopped)
		errorsx.Log(t.init.Wait())
	}()

	if err = idmap(t.init.Process.Pid); err != nil {
		return nil, err
	}

	mapped[1].Close()
	if t.dev, err = received(mounted[0]); err != nil {
		return nil, errorsx.Wrap(err, "the machine did not mount its root")
	}

	t.served = make(chan struct{})
	go func() {
		defer close(t.served)
		errorsx.Log(egiso.ServeFUSE(t.dev, m.root, m.nfs...))
	}()

	return t, nil
}

// received is the descriptor the machine sends over the socket fd.
func received(fd int) (_ *os.File, err error) {
	var (
		b    = make([]byte, 1)
		oob  = make([]byte, unix.CmsgSpace(4))
		oobn int
		msgs []unix.SocketControlMessage
		fds  []int
	)

	if _, oobn, _, _, err = unix.Recvmsg(fd, b, oob, unix.MSG_CMSG_CLOEXEC); err != nil {
		return nil, err
	}
	if msgs, err = unix.ParseSocketControlMessage(oob[:oobn]); err != nil {
		return nil, err
	}
	if len(msgs) != 1 {
		return nil, errorsx.String("the machine sent no descriptor")
	}
	if fds, err = unix.ParseUnixRights(&msgs[0]); err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fds[0]), "fuse"), nil
}

// idmap maps root of the machine's user namespace to the user, and the ids after it to the user's
// subordinate ids, as rootless podman maps a container's.
func idmap(pid int) error {
	var (
		current *user.User
		err     error
	)

	if current, err = user.Current(); err != nil {
		return err
	}

	for _, m := range []struct {
		tool string
		file string
		id   string
	}{
		{tool: "newuidmap", file: "/etc/subuid", id: current.Uid},
		{tool: "newgidmap", file: "/etc/subgid", id: current.Gid},
	} {
		ranges, cause := subids.ParseSubIDFileFilter(m.file, func(s subids.SubID) bool { return s.Name == current.Username || s.Name == current.Uid })
		if cause != nil {
			return errorsx.Wrapf(cause, "unable to read %s", m.file)
		}
		if len(ranges) == 0 {
			return errorsx.Errorf("%s has no ids for %s", m.file, current.Username)
		}
		cmd := exec.Command(m.tool, strconv.Itoa(pid), "0", m.id, "1", "1", strconv.FormatInt(ranges[0].SubID, 10), strconv.FormatInt(ranges[0].Count, 10))
		if b, cause := cmd.CombinedOutput(); cause != nil {
			return errorsx.Wrapf(cause, "unable to map the machine's ids: %s", b)
		}
	}

	return nil
}

// Exec runs req in the machine and waits for it, with no input and its output discarded unless
// options say otherwise. A command that exits with other than 0 returns an ExitError. It waits for
// the machine to serve commands. Cancelling ctx kills the command.
func (t *VM) Exec(ctx context.Context, req *execproxy.ExecRequest, options ...ExecOption) error {
	return command(ctx, t.connect, req, options...)
}

// connect opens the stream of command id. The agent's socket listens from the start, so a
// connection waits there until the agent serves it.
func (t *VM) connect(ctx context.Context, id uint64, stream uint8) (conn net.Conn, err error) {
	var (
		dialer net.Dialer
	)

	if conn, err = dialer.DialContext(ctx, "unix", t.agent); err != nil {
		return nil, errorsx.Wrap(err, "unable to reach the machine's agent")
	}
	if err = WriteHeader(conn, id, stream); err != nil {
		return nil, errorsx.Compact(err, conn.Close())
	}
	return conn, nil
}

// Close stops the machine, which ends every process of its namespaces and with them the mount of its
// root, and waits for its root to stop being served.
func (t *VM) Close() error {
	var (
		errs []error
	)

	if t.init != nil && t.init.Process != nil {
		errorsx.Log(errorsx.Ignore(t.init.Process.Signal(syscall.SIGTERM), os.ErrProcessDone))
		<-t.stopped
	}
	if t.served != nil {
		<-t.served
	}
	if t.dev != nil {
		errs = append(errs, t.dev.Close())
	}
	if t.netns != "" {
		errs = append(errs, errorsx.Ignore(os.Remove(t.netns), os.ErrNotExist))
	}
	if t.dir != "" {
		errs = append(errs, os.RemoveAll(t.dir))
	}
	return errorsx.Compact(errs...)
}
