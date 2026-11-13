package egvm

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	systemd "github.com/coreos/go-systemd/v22/dbus"
	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/wasinet/wasinet/wnetruntime"
	"github.com/godbus/dbus/v5"
	subids "github.com/moby/sys/user"
	"go.podman.io/common/pkg/cgroups"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/unix"
)

// dnsforward is the address pasta answers DNS on inside the machine, as it does for rootless podman.
const dnsforward = "169.254.1.1"

// sshdir is where systemd-ssh-generator binds sshd's AF_UNIX socket, which the host shares into the
// machine so the socket is the host's to dial.
const sshdir = "/run/ssh-unix-local"

// VM is a running machine: the root's init in namespaces of its own, rooted at the egiso.
type VM struct {
	dir     string
	netns   string
	socket  string
	client  *ssh.ClientConfig
	dev     *os.File
	init    *exec.Cmd
	stopped chan struct{}
	served  chan struct{}
	closers []io.Closer
}

// Boot starts the machine as an unprivileged user: its init runs as root of a user namespace mapping
// the user's subordinate ids, in mount, pid, uts, ipc, network and cgroup namespaces of its own,
// rooted at the egiso served over FUSE. pasta connects its network to the host's, as it does for
// rootless podman, in a scope of the user's systemd limited to the machine's cpus and memory. Its
// init's output is copied to console.
func Boot(m Machine, console io.Writer) (_ *VM, err error) {
	var (
		t          = &VM{stopped: make(chan struct{})}
		root       string
		shared     string
		authorized string
		mapped     [2]*os.File
		mounted    [2]int
		args       []string
	)
	defer func() {
		if err != nil {
			errorsx.Log(t.Close())
		}
	}()

	if t.dir, err = os.MkdirTemp("", "egvm.*"); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine's directory")
	}
	root, shared = filepath.Join(t.dir, "root"), filepath.Join(t.dir, "ssh")
	t.socket = filepath.Join(shared, "socket")
	if err = os.Mkdir(root, 0755); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine's root")
	}
	if err = os.Mkdir(shared, 0700); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine's ssh directory")
	}

	if t.client, authorized, err = client(m.root.Upper()); err != nil {
		return nil, err
	}

	// the network namespace file goes where rootless podman keeps its own, which pasta may read.
	t.netns = filepath.Join(envx.String(t.dir, "XDG_RUNTIME_DIR"), "netns", filepath.Base(t.dir))
	if err = os.MkdirAll(filepath.Dir(t.netns), 0700); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the network namespace directory")
	}
	// SELinux policy labels the directory as it does ip netns's, which pasta may read, but one Boot
	// creates takes the runtime directory's label until restorecon applies the policy's.
	if restorecon, cause := exec.LookPath("restorecon"); cause == nil {
		if b, cause := exec.Command(restorecon, filepath.Dir(t.netns)).CombinedOutput(); cause != nil {
			return nil, errorsx.Wrapf(cause, "unable to label the network namespace directory: %s", b)
		}
	}
	if err = os.WriteFile(t.netns, nil, 0600); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine's network namespace file")
	}

	if mapped[0], mapped[1], err = os.Pipe(); err != nil {
		return nil, err
	}
	defer mapped[1].Close()
	if mounted, err = unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0); err != nil {
		return nil, errorsx.Compact(err, mapped[0].Close())
	}
	defer unix.Close(mounted[0])

	args = []string{stagemap, root, m.init, t.netns, authorized, nftables(m.firewall), shared, sshdir, strconv.FormatBool(false)}
	for _, mnt := range m.mounts {
		args = append(args, mnt.dir, mnt.path, strconv.FormatBool(mnt.readonly))
	}

	t.init = &exec.Cmd{
		Path:        "/proc/self/exe",
		Args:        args,
		Stdout:      console,
		Stderr:      console,
		ExtraFiles:  []*os.File{mapped[0], os.NewFile(uintptr(mounted[1]), "mounted")},
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

	// the machine starts nothing else until its ids are mapped, so all of it starts in the scope.
	if err = scope(t.init.Process.Pid, m); err != nil {
		return nil, err
	}

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

// scope moves pid into a transient scope of the user's systemd, as systemd-run --user --scope -p
// Delegate=yes runs a command, limited to the machine's cpus and, unless it is the host's, memory. The
// root's systemd manages the scope's cgroup, which the scope delegates to it.
func scope(pid int, m Machine) (err error) {
	var (
		conn       *systemd.Conn
		done       = make(chan string, 1)
		result     string
		properties = []systemd.Property{
			systemd.PropDescription("eg machine"),
			systemd.PropPids(uint32(pid)),
			{Name: "Delegate", Value: dbus.MakeVariant(true)},
			{Name: "CollectMode", Value: dbus.MakeVariant("inactive-or-failed")},
			{Name: "CPUQuotaPerSecUSec", Value: dbus.MakeVariant(uint64(m.cpus) * uint64(time.Second/time.Microsecond))},
		}
	)

	if m.memory > 0 {
		properties = append(properties, systemd.Property{Name: "MemoryMax", Value: dbus.MakeVariant(m.memory)})
	}

	if conn, err = cgroups.UserConnection(os.Getuid()); err != nil {
		return errorsx.Wrap(err, "unable to reach the user's systemd, which machines run under")
	}
	defer conn.Close()

	if _, err = conn.StartTransientUnitContext(context.Background(), fmt.Sprintf("egvm-%d.scope", pid), "fail", properties, done); err != nil {
		return errorsx.Wrap(err, "unable to start the machine's scope")
	}
	if result = <-done; result != "done" {
		return errorsx.Errorf("unable to start the machine's scope: %s", result)
	}
	return nil
}

// nftables is the ruleset the machine loads to reach only what fw permits, besides its loopback and
// DNS: fw's allowed prefixes, then nothing of its blocked ones. It filters what leaves the machine
// and what the machine forwards for its own containers; nothing enters a machine it did not ask for.
func nftables(fw wnetruntime.Firewall) string {
	var (
		b strings.Builder
	)

	b.WriteString("table inet eg {\n\tchain reach {\n\t\tmeta l4proto { tcp, udp } th dport 53 accept\n")
	addresses(&b, fw.Allow, "accept")
	addresses(&b, fw.Block, "drop")
	b.WriteString("\t}\n\tchain output {\n\t\ttype filter hook output priority filter; policy accept;\n\t\toifname \"lo\" accept\n\t\tjump reach\n\t}\n")
	b.WriteString("\tchain forward {\n\t\ttype filter hook forward priority filter; policy accept;\n\t\tjump reach\n\t}\n}\n")
	return b.String()
}

// addresses writes the rules giving packets to prefixes verdict, one per address family.
func addresses(b *strings.Builder, prefixes []netip.Prefix, verdict string) {
	for _, family := range []struct {
		match string
		is    func(netip.Addr) bool
	}{
		{match: "ip", is: netip.Addr.Is4},
		{match: "ip6", is: func(a netip.Addr) bool { return !a.Is4() }},
	} {
		set := slicesx.Filter(func(p netip.Prefix) bool { return family.is(p.Addr()) }, prefixes...)
		if len(set) == 0 {
			continue
		}
		fmt.Fprintf(b, "\t\t%s daddr { %s } %s\n", family.match, strings.Join(slicesx.MapTransform(netip.Prefix.String, set...), ", "), verdict)
	}
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
func (t *VM) Exec(ctx context.Context, req *execproxy.ExecRequest, options ...ExecOption) (err error) {
	var (
		conn net.Conn
	)

	if conn, err = redial(ctx, func() (net.Conn, error) { return net.Dial("unix", t.socket) }, t.stopped, func() error { return nil }); err != nil {
		return err
	}
	return command(ctx, conn, t.client, req, options...)
}

// Forward makes the host's unix socket host reachable in the machine at path, a socket the machine's
// sshd listens on until the machine stops. It waits for the machine to serve commands.
func (t *VM) Forward(ctx context.Context, host string, path string) (err error) {
	var (
		conn      net.Conn
		forwarded io.Closer
	)

	if conn, err = redial(ctx, func() (net.Conn, error) { return net.Dial("unix", t.socket) }, t.stopped, func() error { return nil }); err != nil {
		return err
	}
	if forwarded, err = forward(conn, t.client, host, path); err != nil {
		return err
	}
	t.closers = append(t.closers, forwarded)
	return nil
}

// Close stops the machine, which ends every process of its namespaces and with them the mount of its
// root, and waits for its root to stop being served.
func (t *VM) Close() error {
	var (
		errs []error
	)

	for _, c := range t.closers {
		errorsx.Log(c.Close())
	}
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
