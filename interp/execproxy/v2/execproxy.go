// Package execproxy is the second version of the exec proxy. it carries a command's stdio over unix sockets
// in the runtime directory, and is only served with EG_EXPERIMENTAL_PROC_SOCKETS.
package execproxy

//go:generate protoc --proto_path=. --go_opt=Meg.interp.exec.v2.proto=github.com/egdaemon/eg/interp/execproxy/v2;execproxy --go_opt=paths=source_relative --go_out=. eg.interp.exec.v2.proto
//go:generate protoc --proto_path=. --go-grpc_opt=Meg.interp.exec.v2.proto=github.com/egdaemon/eg/interp/execproxy/v2;execproxy --go-grpc_opt=paths=source_relative --go-grpc_out=. eg.interp.exec.v2.proto

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
	v1 "github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/runtime/x/wasi/execx"
	"google.golang.org/grpc"
)

// Auto serves v1 alone unless EG_EXPERIMENTAL_PROC_SOCKETS is set.
func Auto(root string, runtimedir string, environ []string) interface{ Bind(grpc.ServiceRegistrar) } {
	if !envx.Boolean(false, eg.EnvExperimentalProcSockets) {
		return v1.NewExecProxy(root, environ)
	}

	return NewProxy(root, runtimedir, environ)
}

func NewProxy(root string, runtimedir string, environ []string) *Proxy {
	return &Proxy{
		v1:         v1.NewExecProxy(root, environ),
		dir:        root,
		runtimedir: runtimedir,
		environ:    environ,
	}
}

type Proxy struct {
	UnimplementedProxyServer
	v1         *v1.ExecProxy
	dir        string
	runtimedir string
	environ    []string
}

// Bind registers v1 alongside this proxy, guests still run commands without stdio through it.
func (t *Proxy) Bind(host grpc.ServiceRegistrar) {
	t.v1.Bind(host)
	RegisterProxyServer(host, t)
}

// Exec runs the command with the guest's sockets in proc/{id} as its stdio and returns its exit code.
func (t *Proxy) Exec(ctx context.Context, req *ExecRequest) (resp *ExecResponse, err error) {
	var (
		cmd   *exec.Cmd = exec.CommandContext(ctx, req.Cmd, req.Arguments...)
		stdio []*os.File
	)

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
func (t *Proxy) stdio(id string) (files []*os.File, err error) {
	var (
		conn net.Conn
		f    *os.File
	)

	if !filepath.IsLocal(id) || filepath.Base(id) != id {
		return nil, errorsx.Errorf("invalid process id: %q", id)
	}

	for _, name := range stdionames {
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
