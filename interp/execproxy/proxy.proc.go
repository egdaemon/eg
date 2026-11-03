package execproxy

import (
	context "context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/x/wasi/execx"
	"google.golang.org/grpc"
)

// Auto returns ProcProxy when EG_EXPERIMENTAL_PROC_SOCKETS is set and ExecProxy otherwise.
func Auto(root string, runtimedir string, environ []string) interface{ Bind(grpc.ServiceRegistrar) } {
	if envx.Boolean(false, eg.EnvExperimentalProcSockets) {
		return NewProcProxy(root, runtimedir, environ)
	}

	return NewExecProxy(root, environ)
}

func NewProcProxy(root string, runtimedir string, environ []string) *ProcProxy {
	return &ProcProxy{
		ExecProxy:  NewExecProxy(root, environ),
		runtimedir: runtimedir,
	}
}

// ProcProxy runs a request with an id with the guest's sockets in proc/{id} as its stdio. requests without
// an id are run by ExecProxy.
type ProcProxy struct {
	*ExecProxy
	runtimedir string
}

func (t *ProcProxy) Bind(host grpc.ServiceRegistrar) {
	RegisterProxyServer(host, t)
}

// Exec runs the command and returns its exit code.
func (t *ProcProxy) Exec(ctx context.Context, req *ExecRequest) (resp *ExecResponse, err error) {
	var (
		cmd   *exec.Cmd = exec.CommandContext(ctx, req.Cmd, req.Arguments...)
		stdio []*os.File
	)

	if req.Id == "" {
		return t.ExecProxy.Exec(ctx, req)
	}

	cmd.Dir = req.Dir
	if !filepath.IsAbs(cmd.Dir) {
		cmd.Dir = filepath.Join(t.dir, cmd.Dir)
	}

	cmd.Env = append(t.environ, req.Environment...)

	if stdio, err = t.stdio(req.Id); err != nil {
		return nil, err
	}
	defer func() {
		for _, f := range stdio {
			f.Close()
		}
	}()

	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdio[0], stdio[1], stdio[2]

	if err = execx.MaybeRun(cmd); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return nil, err
		}
	}

	return &ExecResponse{Exit: exitcode(cmd.ProcessState)}, nil
}

// stdio connects to the guest's stdin, stdout and stderr sockets for the process id. the sockets are handed
// to the command as files, so its output reaches the guest without passing through this process.
func (t *ProcProxy) stdio(id string) (files []*os.File, err error) {
	var (
		conn net.Conn
		f    *os.File
	)

	if !filepath.IsLocal(id) || filepath.Base(id) != id {
		return nil, errorsx.Errorf("invalid process id: %q", id)
	}

	for _, name := range []string{"stdin", "stdout", "stderr"} {
		if conn, err = net.Dial("unix", filepath.Join(t.runtimedir, "proc", id, name)); err != nil {
			break
		}

		f, err = conn.(*net.UnixConn).File()
		conn.Close()
		if err != nil {
			break
		}

		files = append(files, f)
	}

	if err != nil {
		for _, f = range files {
			f.Close()
		}
		return nil, errorsx.Wrapf(err, "unable to connect the stdio of process %s", id)
	}

	return files, nil
}

// exitcode reports a process killed by a signal as 128 plus the signal, like a shell does.
func exitcode(state *os.ProcessState) int32 {
	if ws, ok := state.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int32(ws.Signal())
	}
	return int32(state.ExitCode())
}
