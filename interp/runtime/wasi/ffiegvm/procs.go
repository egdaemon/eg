package ffiegvm

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/gofrs/uuid/v5"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Exec runs req in a machine with stdin, stdout and stderr as its stdio, and returns its exit code.
type Exec func(ctx context.Context, req *execproxy.ExecRequest, stdin io.Reader, stdout io.Writer, stderr io.Writer) (int32, error)

// Procs runs the commands a guest describes in proc/{id} of its runtime directory in a machine, the way the
// egcoreutils reactor runs utilities: proc/{id}/cmdline, environ and cwd describe the command, and its stdio are
// the unix sockets Procs listens on in proc/{id}/fd.
type Procs struct {
	ctx        context.Context
	runtimedir string
	exec       Exec
	mu         sync.Mutex
	running    map[uint64]*proc
}

type proc struct {
	done chan struct{}
	code int32
	err  error
}

// NewProcs runs commands with exec, until ctx is done, for a guest whose runtime directory is runtimedir on the
// host. without exec the guest is told the host does not implement them.
func NewProcs(ctx context.Context, runtimedir string, exec Exec) *Procs {
	return &Procs{ctx: ctx, runtimedir: runtimedir, exec: exec, running: make(map[uint64]*proc)}
}

// Export adds Init and Wait to host, under the names the guest imports them by.
func (t *Procs) Export(host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
	return host.
		NewFunctionBuilder().WithFunc(t.Init).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegvm.Execveatinit").
		NewFunctionBuilder().WithFunc(t.Wait).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegvm.Execveatwait")
}

// Init starts the command described in proc/{id} and listens for the guest's connections to its stdio.
func (t *Procs) Init(ctx context.Context, m api.Module, id uint64) uint32 {
	var (
		u         uuid.UUID
		req       execproxy.ExecRequest
		l         net.Listener
		listeners []net.Listener
		err       error
	)

	if t.exec == nil {
		return ffierrors.ErrNotImplemented
	}

	// the guest names the directory after the id, as egunsafe.PatternIsolateFrom formats it.
	binary.BigEndian.PutUint64(u[8:], id)
	dir := filepath.Join(t.runtimedir, "proc", egunsafe.PatternIsolateFrom(u, 8, "%s"))

	read := func(name string) []string {
		b, cause := os.ReadFile(filepath.Join(dir, name))
		err = errorsx.Compact(err, cause)
		if len(b) == 0 {
			return nil
		}
		return strings.Split(string(b), "\x00")
	}

	cmdline, environ, cwd := read("cmdline"), read("environ"), read("cwd")
	if err != nil || len(cmdline) == 0 || len(cwd) != 1 {
		log.Println("unable to read the command", dir, err)
		return ffierrors.ErrUnrecoverable
	}
	req = execproxy.ExecRequest{Cmd: cmdline[0], Arguments: cmdline[1:], Environment: environ, Dir: cwd[0]}

	for _, fd := range []string{"0", "1", "2"} {
		if l, err = net.Listen("unix", filepath.Join(dir, "fd", fd)); err != nil {
			log.Println("unable to listen for the command's stdio", err)
			for _, l = range listeners {
				l.Close()
			}
			return ffierrors.ErrUnrecoverable
		}
		listeners = append(listeners, l)
	}

	p := &proc{done: make(chan struct{})}
	t.mu.Lock()
	t.running[id] = p
	t.mu.Unlock()

	go func() {
		var (
			conn  net.Conn
			conns []net.Conn
			cause error
		)
		defer close(p.done)
		defer func() {
			for _, c := range conns {
				c.Close()
			}
		}()

		for _, l := range listeners {
			conn, cause = l.Accept()
			l.Close()
			if cause != nil {
				p.err = errorsx.Wrap(cause, "unable to accept the command's stdio")
				return
			}
			conns = append(conns, conn)
		}

		p.code, p.err = t.exec(t.ctx, &req, conns[0], conns[1], conns[2])
	}()

	return 0
}

// Wait reports the exit code of the command started for id, or ffierrors.ErrPending while it runs.
func (t *Procs) Wait(ctx context.Context, m api.Module, id uint64) uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()

	p, ok := t.running[id]
	if !ok {
		return ffierrors.ErrNotFound
	}

	select {
	case <-p.done:
	default:
		return ffierrors.ErrPending
	}

	delete(t.running, id)
	if p.err != nil {
		log.Println("unable to run the command in the machine", p.err)
		return ffierrors.ErrUnrecoverable
	}

	return uint32(p.code)
}
