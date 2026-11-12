package egvm

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/egdaemon/eg/interp/execproxy"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/encoding/protodelim"
)

// defaultpath is the PATH of commands whose environment does not set one.
const defaultpath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// Answering returns once pid 1 is rootinit and, when rootinit is systemd, once systemd listens on
// the private socket systemctl talks to it over; until then systemctl reports the machine offline.
func Answering(rootinit string) error {
	var (
		want os.FileInfo
		pid1 os.FileInfo
		exe  string
		err  error
	)

	if want, err = os.Stat(rootinit); err != nil {
		return fmt.Errorf("unable to find the root's init %s: %w", rootinit, err)
	}
	for pid1, err = os.Stat("/proc/1/exe"); err != nil || !os.SameFile(pid1, want); pid1, err = os.Stat("/proc/1/exe") {
		time.Sleep(10 * time.Millisecond)
	}

	if exe, err = filepath.EvalSymlinks(rootinit); err != nil {
		return err
	}
	if filepath.Base(exe) != "systemd" {
		return nil
	}
	for _, err = os.Stat("/run/systemd/private"); err != nil; _, err = os.Stat("/run/systemd/private") {
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// pending collects the connections of a command as they arrive.
type pending struct {
	streams [4]*os.File
	count   int
}

// Serve accepts connections on the listening socket fd and runs each command once all of its
// streams have connected.
func Serve(fd int) error {
	var (
		mu       sync.Mutex
		commands = make(map[uint64]*pending)
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
			h, cause := ReadHeader(conn)
			if cause != nil || h.Stream > StreamControl {
				fmt.Println("egvm: invalid connection", h, cause)
				conn.Close()
				return
			}

			mu.Lock()
			c, ok := commands[h.ID]
			if !ok {
				c = &pending{}
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
		}(os.NewFile(uintptr(nfd), "agent"))
	}
}

// execute runs the command its control stream requests with the other streams as its stdio, and
// responds with its exit code. A command that can not start exits with 127, like a shell's.
func execute(streams [4]*os.File) {
	var (
		req     execproxy.ExecRequest
		control = streams[StreamControl]
		reader  = bufio.NewReader(control)
		code    int32
	)
	defer control.Close()

	if err := protodelim.UnmarshalFrom(reader, &req); err != nil {
		fmt.Println("egvm: invalid request", err)
		for _, s := range streams[:StreamControl] {
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
	cmd.Stdin, cmd.Stdout, cmd.Stderr = streams[StreamStdin], streams[StreamStdout], streams[StreamStderr]

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(streams[StreamStderr], err)
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
	for _, s := range streams[:StreamControl] {
		s.Close()
	}

	if _, err := protodelim.MarshalTo(control, &execproxy.ExecResponse{Exit: code}); err != nil {
		fmt.Println("egvm: unable to respond", err)
	}
}

// lookpath finds name in the PATH of environ, rather than the agent's own. A name with a slash, or
// one no directory holds, is returned as it is, and fails to start.
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
