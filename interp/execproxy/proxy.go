package execproxy

import (
	context "context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/x/wasi/execx"
	"google.golang.org/grpc"
)

func NewExecProxy(root string, runtimedir string, environ []string) *ExecProxy {
	return &ExecProxy{
		dir:        root,
		runtimedir: runtimedir,
		environ:    environ,
	}
}

type ExecProxy struct {
	UnimplementedProxyServer
	dir        string
	runtimedir string
	environ    []string
}

func (t *ExecProxy) Bind(host grpc.ServiceRegistrar) {
	RegisterProxyServer(host, t)
}

// Exec runs the command. With an id its stdio is the guest's sockets in proc/{id} and its exit code is
// returned; without one it inherits the host's stdio and a failed exit is an error.
func (t *ExecProxy) Exec(ctx context.Context, req *ExecRequest) (resp *ExecResponse, err error) {
	var (
		cmd   *exec.Cmd = exec.CommandContext(ctx, req.Cmd, req.Arguments...)
		stdio []*os.File
	)

	cmd.Dir = req.Dir
	if !filepath.IsAbs(cmd.Dir) {
		cmd.Dir = filepath.Join(t.dir, cmd.Dir)
	}

	cmd.Env = append(t.environ, req.Environment...)

	if req.Id == "" {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err = execx.MaybeRun(cmd); err != nil {
			return nil, err
		}

		return &ExecResponse{}, nil
	}

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
func (t *ExecProxy) stdio(id string) (files []*os.File, err error) {
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
