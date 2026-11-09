//go:build linux

// Command vminit is the init of an egvm machine. It loads the kernel modules in /modules in name
// order, mounts the virtio-fs share tagged root, chroots into it, and serves commands on vsock port
// egvm.Port until the host stops the machine.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/interp/execproxy"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/protodelim"
)

func main() {
	fmt.Println("egvm:", run())
	unix.Sync()
	if err := unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF); err != nil {
		fmt.Println("egvm: unable to power off", err)
	}
}

func run() error {
	var (
		fd int
	)

	for _, m := range []struct{ source, target, fstype string }{
		{"proc", "/proc", "proc"},
		{"sysfs", "/sys", "sysfs"},
		{"devtmpfs", "/dev", "devtmpfs"},
	} {
		if err := mount(m.source, m.target, m.fstype); err != nil {
			return err
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

	if err = mount("root", "/root", "virtiofs"); err != nil {
		return err
	}
	if err = unix.Chroot("/root"); err != nil {
		return fmt.Errorf("unable to chroot into the share: %w", err)
	}
	if err = unix.Chdir("/"); err != nil {
		return err
	}

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
		dirs = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
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

func mount(source string, target string, fstype string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	if err := unix.Mount(source, target, fstype, 0, ""); err != nil {
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
