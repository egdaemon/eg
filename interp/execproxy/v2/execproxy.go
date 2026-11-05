// Package execproxy is the second version of the exec proxy. it carries a command's stdio over unix sockets
// in the runtime directory, and is only served with EG_EXPERIMENTAL_PROC_SOCKETS.
package execproxy

import (
	context "context"
	"crypto/rand"
	"errors"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	v1 "github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/runtime/x/wasi/execx"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
	"google.golang.org/grpc"
)

// Coreutils is where the egcoreutils package installs uutils' coreutils.wasm.
const Coreutils = "/usr/share/eg/coreutils.wasm"

// Auto returns v2 when EG_EXPERIMENTAL_PROC_SOCKETS is set and v1 otherwise. mounts are the guest's, which
// v2 gives to the WASM modules it runs.
func Auto(root string, runtimedir string, environ []string, mounts wazero.FSConfig) interface{ Bind(grpc.ServiceRegistrar) } {
	if envx.Boolean(false, eg.EnvExperimentalProcSockets) {
		return NewProxy(root, runtimedir, environ, mounts)
	}

	return v1.NewExecProxy(root, environ)
}

func NewProxy(root string, runtimedir string, environ []string, mounts wazero.FSConfig) *Proxy {
	runtime := wazero.NewRuntimeWithConfig(context.Background(), wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	wasi_snapshot_preview1.MustInstantiate(context.Background(), runtime)

	return &Proxy{
		dir:        root,
		runtimedir: runtimedir,
		environ:    environ,
		mounts:     mounts,
		runtime:    runtime,
		wasms: map[string]func() (wazero.CompiledModule, error){
			"coreutils": compiler(runtime, Coreutils),
		},
	}
}

type Proxy struct {
	v1.UnimplementedProxyServer
	dir        string
	runtimedir string
	environ    []string
	mounts     wazero.FSConfig
	runtime    wazero.Runtime
	wasms      map[string]func() (wazero.CompiledModule, error)
}

// compiler compiles the module at path once, on first use.
func compiler(runtime wazero.Runtime, path string) func() (wazero.CompiledModule, error) {
	return sync.OnceValues(func() (_ wazero.CompiledModule, err error) {
		var (
			bin []byte
		)

		if bin, err = os.ReadFile(path); err != nil {
			return nil, err
		}

		return runtime.CompileModule(context.Background(), bin)
	})
}

func (t *Proxy) Bind(host grpc.ServiceRegistrar) {
	v1.RegisterProxyServer(host, t)
}

// Exec runs the command with the guest's sockets in proc/{id} as its stdio and returns its exit code. a
// request without an id comes from a guest that predates v2: the command inherits the module's stdio and a
// failed exit is an error, which is how those guests detect failure.
func (t *Proxy) Exec(ctx context.Context, req *v1.ExecRequest) (resp *v1.ExecResponse, err error) {
	var (
		cmd      *exec.Cmd = exec.CommandContext(ctx, req.Cmd, req.Arguments...)
		stdio    []*os.File
		compiled wazero.CompiledModule
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

	if stdio, err = t.stdio(req.Id); err != nil {
		return nil, err
	}
	defer func() {
		for _, f := range stdio {
			f.Close()
		}
	}()

	if compiled, err = t.wasm(req.Wasm); err != nil {
		return nil, err
	}

	if compiled != nil {
		return t.module(ctx, compiled, cmd.Dir, req, stdio)
	}

	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdio[0], stdio[1], stdio[2]

	if err = execx.MaybeRun(cmd); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return nil, err
		}
	}

	return &v1.ExecResponse{Exit: exitcode(cmd.ProcessState)}, nil
}

// wasm returns the compiled module the request names. it returns nil for a request without one, and when the
// module is not installed, so the command runs natively.
func (t *Proxy) wasm(name string) (compiled wazero.CompiledModule, err error) {
	var (
		compile, ok = t.wasms[name]
	)

	if name == "" {
		return nil, nil
	}

	if !ok {
		return nil, errorsx.Errorf("unknown wasm module: %q", name)
	}

	if compiled, err = compile(); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	return compiled, err
}

// module runs the command in the compiled module with the guest's mounts. the module has no working directory,
// so the command's directory is mounted at . and relative paths resolve within it. wazero treats . as /, so it
// also replaces the guest's empty root.
func (t *Proxy) module(ctx context.Context, compiled wazero.CompiledModule, dir string, req *v1.ExecRequest, stdio []*os.File) (resp *v1.ExecResponse, err error) {
	var (
		mod api.Module
	)

	cfg := wazero.NewModuleConfig().
		WithName("").
		WithArgs(append([]string{req.Cmd}, req.Arguments...)...).
		WithStdin(stdio[0]).
		WithStdout(stdio[1]).
		WithStderr(stdio[2]).
		WithFSConfig(t.mounts.WithDirMount(dir, ".")).
		WithSysWalltime().
		WithSysNanotime().
		WithRandSource(rand.Reader)

	for _, kv := range append(t.environ, req.Environment...) {
		if k, v, ok := strings.Cut(kv, "="); ok {
			cfg = cfg.WithEnv(k, v)
		}
	}

	if mod, err = t.runtime.InstantiateModule(ctx, compiled, cfg); mod != nil {
		mod.Close(ctx)
	}

	if exit, ok := errors.AsType[*sys.ExitError](err); ok {
		return &v1.ExecResponse{Exit: int32(exit.ExitCode())}, nil
	}

	if err != nil {
		return nil, err
	}

	return &v1.ExecResponse{}, nil
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
