// Package execproxy is the second version of the exec proxy. it carries a command's stdio over unix sockets
// in the runtime directory, and is only served with EG_EXPERIMENTAL_PROC_SOCKETS.
package execproxy

import (
	context "context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	v1 "github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/interp/execproxy/v2/proc"
	"github.com/egdaemon/eg/runtime/x/wasi/execx"
	"google.golang.org/grpc"
)

// Auto returns v2 when EG_EXPERIMENTAL_PROC_SOCKETS is set and v1 otherwise.
func Auto(root string, runtimedir string, environ []string) interface{ Bind(grpc.ServiceRegistrar) } {
	if envx.Boolean(false, eg.EnvExperimentalProcSockets) {
		return NewProxy(root, runtimedir, environ)
	}

	return v1.NewExecProxy(root, environ)
}

func NewProxy(root string, runtimedir string, environ []string) *Proxy {
	return &Proxy{
		dir:        root,
		runtimedir: runtimedir,
		environ:    environ,
	}
}

type Proxy struct {
	v1.UnimplementedProxyServer
	dir        string
	runtimedir string
	environ    []string
}

func (t *Proxy) Bind(host grpc.ServiceRegistrar) {
	v1.RegisterProxyServer(host, t)
}

// Exec runs the command with the guest's sockets in proc/{id} as its stdio and returns its exit code. a
// request without an id comes from a guest that predates v2: the command inherits the module's stdio and a
// failed exit is an error, which is how those guests detect failure.
func (t *Proxy) Exec(ctx context.Context, req *v1.ExecRequest) (resp *v1.ExecResponse, err error) {
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
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err = execx.MaybeRun(cmd); err != nil {
			return nil, err
		}
		return &v1.ExecResponse{}, nil
	}

	if stdio, err = proc.Dial(t.runtimedir, req.Id); err != nil {
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

	return &v1.ExecResponse{Exit: exitcode(cmd.ProcessState)}, nil
}

// exitcode reports a process killed by a signal as 128 plus the signal, like a shell does.
func exitcode(state *os.ProcessState) int32 {
	if ws, ok := state.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int32(ws.Signal())
	}
	return int32(state.ExitCode())
}
