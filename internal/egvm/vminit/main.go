//go:build linux

// Command vminit is the init of an egvm machine. It loads the kernel modules in /modules in name
// order, mounts the root the host serves over NFS, and execs the root's init, the init= of the
// kernel's command line, as the machine's pid 1. Before it does, it starts itself again twice; both
// outlive the switch. One forwards the guest's NFS client to the host over vsock. The other, the
// agent, serves commands on vsock port egvm.Port inside the root.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/interp/execproxy"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/protodelim"
)

// defaultpath is the PATH of commands whose environment does not set one.
const defaultpath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

func main() {
	// boot starts vminit again under these names, with the descriptors each needs.
	switch os.Args[0] {
	case "nfs":
		fmt.Println("egvm: nfs:", forward(os.NewFile(3, "nfs")))
		os.Exit(1)
	case "agent":
		fmt.Println("egvm: agent:", agent(os.NewFile(3, "switch"), os.NewFile(4, "switched")))
		os.Exit(1)
	}

	fmt.Println("egvm:", boot())
	unix.Sync()
	if err := unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF); err != nil {
		fmt.Println("egvm: unable to power off", err)
	}
}

// boot only returns when the machine fails to start the root's init.
func boot() error {
	var (
		rootinit = "/usr/sbin/init"
		nfs      net.Listener
		listener *os.File
		cmdline  []byte
	)

	for _, m := range []struct{ source, target, fstype string }{
		{"proc", "/proc", "proc"},
		{"sysfs", "/sys", "sysfs"},
		{"devtmpfs", "/dev", "devtmpfs"},
	} {
		if err := mount(m.source, m.target, m.fstype, ""); err != nil {
			return err
		}
	}

	cmdline, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return err
	}
	for _, param := range strings.Fields(string(cmdline)) {
		if v, ok := strings.CutPrefix(param, "init="); ok {
			rootinit = v
		}
	}

	modules, err := filepath.Glob("/modules/*.ko")
	if err != nil {
		return err
	}
	for _, m := range modules {
		if err = load(m); err != nil {
			return err
		}
	}

	if err = loopback(); err != nil {
		return err
	}
	if nfs, err = net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(egvm.NFSPort))); err != nil {
		return fmt.Errorf("unable to listen for nfs: %w", err)
	}
	if listener, err = nfs.(*net.TCPListener).File(); err != nil {
		return err
	}

	switching, switchw, err := os.Pipe()
	if err != nil {
		return err
	}
	switchedr, switched, err := os.Pipe()
	if err != nil {
		return err
	}

	// the forwarder never forks, so the commands the agent forks always have it to page themselves in over NFS.
	for _, child := range []*exec.Cmd{
		{Path: "/init", Args: []string{"nfs"}, Stdout: os.Stdout, Stderr: os.Stderr, ExtraFiles: []*os.File{listener}},
		{Path: "/init", Args: []string{"agent"}, Stdout: os.Stdout, Stderr: os.Stderr, ExtraFiles: []*os.File{switching, switched}},
	} {
		if err = child.Start(); err != nil {
			return fmt.Errorf("unable to start %s: %w", child.Args[0], err)
		}
	}
	listener.Close()
	switching.Close()
	switched.Close()

	options := fmt.Sprintf("vers=3,proto=tcp,port=%d,mountport=%d,mountproto=tcp,nolock,addr=127.0.0.1,mountaddr=127.0.0.1", egvm.NFSPort, egvm.NFSPort)
	if err = mount("127.0.0.1:/", "/root", "nfs", options); err != nil {
		return err
	}
	for _, dir := range []string{"/dev", "/proc", "/sys"} {
		if err = unix.Mount(dir, "/root"+dir, "", unix.MS_MOVE, ""); err != nil {
			return fmt.Errorf("unable to move %s into the root: %w", dir, err)
		}
	}

	// the root sees /sys read only, as a container does, so its systemd configures devices without
	// the udev container images leave out.
	if err = unix.Mount("", "/root/sys", "", unix.MS_REMOUNT|unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("unable to make the root's /sys read only: %w", err)
	}

	// the agent enters the root before it moves over the initramfs, which hides the root's path.
	switchw.Close()
	if _, err = io.Copy(io.Discard, switchedr); err != nil {
		return err
	}

	if err = unix.Chdir("/root"); err != nil {
		return err
	}
	if err = unix.Mount(".", "/", "", unix.MS_MOVE, ""); err != nil {
		return fmt.Errorf("unable to move the root over the initramfs: %w", err)
	}
	if err = unix.Chroot("."); err != nil {
		return err
	}
	if err = unix.Chdir("/"); err != nil {
		return err
	}

	return fmt.Errorf("unable to exec %s: %w", rootinit, unix.Exec(rootinit, []string{rootinit}, os.Environ()))
}

// agent enters the root once switching closes, closes switched, and serves commands.
func agent(switching *os.File, switched *os.File) (err error) {
	var (
		fd int
	)

	if _, err = io.Copy(io.Discard, switching); err != nil {
		return err
	}
	if err = unix.Chroot("/root"); err != nil {
		return fmt.Errorf("unable to enter the root: %w", err)
	}
	if err = unix.Chdir("/"); err != nil {
		return err
	}
	switched.Close()

	if fd, err = unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0); err != nil {
		return fmt.Errorf("unable to create the vsock listener: %w", err)
	}
	if err = unix.Bind(fd, &unix.SockaddrVM{CID: unix.VMADDR_CID_ANY, Port: egvm.Port}); err != nil {
		return fmt.Errorf("unable to bind vsock port %d: %w", egvm.Port, err)
	}
	if err = unix.Listen(fd, unix.SOMAXCONN); err != nil {
		return fmt.Errorf("unable to listen on vsock port %d: %w", egvm.Port, err)
	}

	return serve(fd)
}

// forward connects each connection the listener nfs accepts to the host's NFS server.
func forward(nfs *os.File) (err error) {
	var (
		l    net.Listener
		conn net.Conn
	)

	if l, err = net.FileListener(nfs); err != nil {
		return err
	}

	for {
		if conn, err = l.Accept(); err != nil {
			return err
		}

		go func(conn net.Conn) {
			defer conn.Close()

			fd, cause := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
			if cause != nil {
				fmt.Println("egvm: unable to create an nfs connection", cause)
				return
			}
			host := os.NewFile(uintptr(fd), "nfs")
			defer host.Close()
			if cause = unix.Connect(fd, &unix.SockaddrVM{CID: unix.VMADDR_CID_HOST, Port: egvm.NFSPort}); cause != nil {
				fmt.Println("egvm: unable to connect to the host's nfs", cause)
				return
			}

			go func() {
				io.Copy(host, conn)
				host.Close()
			}()
			io.Copy(conn, host)
		}(conn)
	}
}

// loopback brings up lo, which the guest's NFS client reaches the agent on.
func loopback() error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	ifr, err := unix.NewIfreq("lo")
	if err != nil {
		return err
	}
	ifr.SetUint16(unix.IFF_UP | unix.IFF_LOOPBACK | unix.IFF_RUNNING)
	if err = unix.IoctlIfreq(fd, unix.SIOCSIFFLAGS, ifr); err != nil {
		return fmt.Errorf("unable to bring up lo: %w", err)
	}
	return nil
}

// command collects the connections of a command as they arrive.
type command struct {
	streams [4]*os.File
	count   int
}

// serve accepts connections on the listening socket fd and runs each command once all of its
// streams have connected.
func serve(fd int) error {
	var (
		mu       sync.Mutex
		commands = make(map[uint64]*command)
	)

	for {
		nfd, _, err := unix.Accept4(fd, unix.SOCK_CLOEXEC)
		if errors.Is(err, unix.EINTR) || errors.Is(err, unix.ECONNABORTED) {
			continue
		}
		if err != nil {
			return fmt.Errorf("unable to accept a connection: %w", err)
		}

		go func(conn *os.File) {
			h, cause := egvm.ReadHeader(conn)
			if cause != nil || h.Stream > egvm.StreamControl {
				fmt.Println("egvm: invalid connection", h, cause)
				conn.Close()
				return
			}

			mu.Lock()
			c, ok := commands[h.ID]
			if !ok {
				c = &command{}
				commands[h.ID] = c
			}
			c.streams[h.Stream] = conn
			c.count++
			if c.count < len(c.streams) {
				mu.Unlock()
				return
			}
			delete(commands, h.ID)
			mu.Unlock()

			execute(c.streams)
		}(os.NewFile(uintptr(nfd), "vsock"))
	}
}

// execute runs the command its control stream requests with the other streams as its stdio, and
// responds with its exit code. A command that can not start exits with 127, like a shell's.
func execute(streams [4]*os.File) {
	var (
		req     execproxy.ExecRequest
		control = streams[egvm.StreamControl]
		reader  = bufio.NewReader(control)
		code    int32
	)
	defer control.Close()

	if err := protodelim.UnmarshalFrom(reader, &req); err != nil {
		fmt.Println("egvm: invalid request", err)
		for _, s := range streams[:egvm.StreamControl] {
			s.Close()
		}
		return
	}

	cmd := &exec.Cmd{Path: lookpath(req.Cmd, req.Environment), Args: append([]string{req.Cmd}, req.Arguments...)}
	cmd.Dir = req.Dir
	cmd.Env = req.Environment
	if !slices.ContainsFunc(cmd.Env, func(kv string) bool { return strings.HasPrefix(kv, "PATH=") }) {
		cmd.Env = append(cmd.Env, "PATH="+defaultpath)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = streams[egvm.StreamStdin], streams[egvm.StreamStdout], streams[egvm.StreamStderr]

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(streams[egvm.StreamStderr], err)
		code = 127
	} else {
		// the host closing the control stream before the response kills the command.
		go func() {
			reader.ReadByte()
			cmd.Process.Kill()
		}()
		code = exitcode(cmd.Wait())
	}

	// the command holds its own copies of its stdio, so its streams end when it and its children exit.
	for _, s := range streams[:egvm.StreamControl] {
		s.Close()
	}

	if _, err := protodelim.MarshalTo(control, &execproxy.ExecResponse{Exit: code}); err != nil {
		fmt.Println("egvm: unable to respond", err)
	}
}

// lookpath finds name in the PATH of environ, rather than vminit's own. A name with a slash, or one
// no directory holds, is returned as it is, and fails to start.
func lookpath(name string, environ []string) string {
	var (
		dirs = defaultpath
	)

	if strings.Contains(name, "/") {
		return name
	}

	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok {
			dirs = v
		}
	}

	for _, dir := range filepath.SplitList(dirs) {
		candidate := path.Join(dir, name)
		if st, err := os.Stat(candidate); err == nil && st.Mode().IsRegular() && st.Mode()&0111 != 0 {
			return candidate
		}
	}

	return name
}

// exitcode reports a command killed by a signal as 128 plus the signal, like a shell does.
func exitcode(err error) int32 {
	var (
		exit *exec.ExitError
	)

	if !errors.As(err, &exit) {
		return 0
	}
	if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int32(ws.Signal())
	}
	return int32(exit.ExitCode())
}

func mount(source string, target string, fstype string, options string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	if err := unix.Mount(source, target, fstype, 0, options); err != nil {
		return fmt.Errorf("unable to mount %s at %s: %w", fstype, target, err)
	}
	return nil
}

func load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = unix.FinitModule(int(f.Fd()), "", 0); err != nil && !errors.Is(err, unix.EEXIST) {
		return fmt.Errorf("unable to load %s: %w", path, err)
	}
	return nil
}
