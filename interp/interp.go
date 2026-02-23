package interp

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiegcontainer"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiexec"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigit"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigraph"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiwasinet"
	"github.com/egdaemon/eg/interp/wasidebug"
	"github.com/egdaemon/eg/workspaces"
	"github.com/egdaemon/wasinet/wasinet/wnetruntime"
	"github.com/gofrs/uuid/v5"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/experimental/logging"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/grpc"
)

type Option func(*runner)

func OptionEnviron(s ...string) Option {
	return func(r *runner) {
		r.environ = s
	}
}

type runtimefn func(r runner, host wazero.HostModuleBuilder) wazero.HostModuleBuilder

// Remote uses the api to implement particular actions like building and running containers.
// may not be necessary.
func Remote(ctx context.Context, wshost workspaces.Context, aid string, runid string, svc grpc.ClientConnInterface, module string, options ...Option) error {
	var (
		r = runner{
			initonce: &sync.Once{},
		}
	)
	for _, opt := range options {
		opt(&r)
	}

	debugx.Println("interp workspace context", spew.Sdump(wshost))

	containers := c8s.NewProxyClient(svc)

	runtimeenv := func(r runner, host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
		return host.
			NewFunctionBuilder().WithFunc(ffigraph.NoopTrace).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Trace").
			NewFunctionBuilder().WithFunc(ffigraph.Analysing(false)).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Analysing").
			NewFunctionBuilder().WithFunc(ffigraph.NoopTraceEventPush).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Push").
			NewFunctionBuilder().WithFunc(ffigraph.NoopTraceEventPop).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Pop").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Pull(func(ctx context.Context, name, wdir string, options ...string) (err error) {
			_, err = containers.Pull(ctx, &c8s.PullRequest{
				Name:    name,
				Dir:     wdir,
				Options: options,
			})
			if err != nil {
				return err
			}
			return nil
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Pull").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Build(func(ctx context.Context, name, wdir string, definition string, options ...string) (err error) {
			_, err = containers.Build(ctx, &c8s.BuildRequest{
				Name:       name,
				Directory:  wdir,
				Definition: definition,
				Options:    options,
			})
			if err != nil {
				return err
			}
			return nil
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Build").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Module(func(ctx context.Context, name, modulepath string, options ...string) (err error) {
			cname := fmt.Sprintf("%s.%s", name, md5x.String(modulepath+runid))
			options = append(
				options,
				"--volume", fmt.Sprintf("%s:%s:ro", filepath.Join(wshost.Root, modulepath), eg.ModuleMount()),
			)

			_, err = containers.Module(ctx, &c8s.ModuleRequest{
				Image:   name,
				Name:    cname,
				Mdir:    wshost.RuntimeDir,
				Options: options,
			})
			if err != nil {
				return err
			}
			return nil
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Module").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Run(func(ctx context.Context, name, modulepath string, cmd []string, options ...string) (err error) {
			cname := fmt.Sprintf("%s.%s", name, md5x.String(modulepath+runid))
			_, err = containers.Run(ctx, &c8s.RunRequest{
				Image:   name,
				Name:    cname,
				Command: cmd,
				Options: options,
			})
			if err != nil {
				return err
			}
			return nil
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Run").
			NewFunctionBuilder().WithFunc(ffiexec.Exec(func(cmd *exec.Cmd) *exec.Cmd {
			if !filepath.IsAbs(cmd.Dir) {
				cmd.Dir = filepath.Join(wshost.WorkingDir, cmd.Dir)
			}
			cmd.Env = append(r.environ, cmd.Env...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			return cmd
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiexec.Command").
			NewFunctionBuilder().WithFunc(
			ffigit.Commitish(wshost.WorkingDir),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Commitish").
			NewFunctionBuilder().WithFunc(
			ffigit.CloneV2(wshost.WorkingDir, wshost.RuntimeDir),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.CloneV2").
			NewFunctionBuilder().WithFunc(
			// ffimetric.Metric(evtclient),
			ffigraph.NoopTrace,
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/metrics.Record").
			NewFunctionBuilder().WithFunc(
			ffigraph.NoopTrace,
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/coverage.Report")
	}

	return r.perform(ctx, wshost, runid, module, runtimeenv)
}

type runner struct {
	environ  []string
	initonce *sync.Once
}

func (t runner) perform(ctx context.Context, wshost workspaces.Context, runid, path string, rtb runtimefn) (err error) {
	const (
		DefaultSSLCertDir = "/etc/ssl/certs"
	)

	defer func(before bool) {
		// strictly speaking stdin should remain blocking at all times but using before
		if nonBlocking(os.Stdin.Fd()) == before {
			debugx.Println("---------------------------------------------- stdin was munged ----------------------------------------------")
		}
	}(nonBlocking(os.Stdin.Fd()))
	tracedebug := envx.Boolean(false, eg.EnvLogsTrace)

	ctx = experimental.WithCompilationWorkers(ctx, runtime.GOMAXPROCS(0))

	cache, err := wazero.NewCompilationCacheWithDir(wshost.CacheDirWazero)
	if err != nil {
		return err
	}
	defer cache.Close(ctx)

	if tracedebug {
		ctx = experimental.WithFunctionListenerFactory(ctx,
			logging.NewHostLoggingListenerFactory(os.Stderr, logging.LogScopeFilesystem))
	}

	// Create a new WebAssembly Runtime.
	runtime := wazero.NewRuntimeWithConfig(
		ctx,
		wazero.NewRuntimeConfig().WithCompilationCache(cache),
	)

	inpr, inpw := io.Pipe()
	defer inpr.Close()
	defer inpw.CloseWithError(io.EOF)
	go func() {
		_, _err := io.Copy(inpw, os.Stdin)
		_err = errorsx.Ignore(_err, io.ErrClosedPipe)
		errorsx.Log(errorsx.Wrap(_err, "failed copying stdin"))
		inpw.CloseWithError(_err)
	}()

	outpr, outpw := io.Pipe()
	defer outpr.Close()
	defer outpw.CloseWithError(io.EOF)
	go func() {
		_, _err := io.Copy(os.Stdout, outpr)
		_err = errorsx.Ignore(_err, io.ErrClosedPipe)
		errorsx.Log(errorsx.Wrapf(_err, "failed copying to stdout: %T", _err))
		outpw.CloseWithError(_err)
	}()

	errpr, errpw := io.Pipe()
	defer errpr.Close()
	defer errpw.CloseWithError(io.EOF)
	go func() {
		_, _err := io.Copy(os.Stderr, errpr)
		_err = errorsx.Ignore(_err, io.ErrClosedPipe)
		errorsx.Log(errorsx.Wrap(_err, "failed copying to stderr"))
		errpw.CloseWithError(_err)
	}()

	hostsslcerts := langx.FirstNonZero(fsx.LocatePhysicalPath("/etc/ssl/certs", "/etc/pki/tls/certs", "/usr/share/ca-certificates"), "/etc/ssl/certs")

	debugx.Println("workload dir", wshost.Root, "->", eg.DefaultWorkloadDirectory())
	debugx.Println("cache dir", wshost.CacheDir, "->", eg.DefaultCacheDirectory())
	debugx.Println("runtime dir", wshost.RuntimeDir, "->", eg.DefaultRuntimeDirectory())
	debugx.Println("working dir", wshost.WorkingDir, "->", eg.DefaultWorkingDirectory())
	debugx.Println("workspace dir", wshost.WorkspaceDir, "->", eg.DefaultWorkspaceDirectory())
	debugx.Println("wazero cache", wshost.CacheDirWazero)
	debugx.Println("system tls certs", hostsslcerts)
	debugx.Println("------------------------------------------------------")

	// we map twice so that baremetal can work. shell commands are run on the host itself so we need the host path.
	// but inside wazero we need to be able to access the standard paths.
	// the call decide which paths the environment variables rsolve to.
	// we also need to ensure we mount the working directory so pwd works correctly.
	wazerofs := wazero.NewFSConfig().
		WithDirMount(hostsslcerts, DefaultSSLCertDir).
		WithDirMount(os.TempDir(), os.TempDir()).
		WithDirMount(wshost.Root, eg.DefaultWorkloadDirectory()).
		WithDirMount(wshost.Root, wshost.Root).
		WithDirMount(wshost.RuntimeDir, eg.DefaultRuntimeDirectory()).
		WithDirMount(wshost.RuntimeDir, wshost.RuntimeDir).
		WithDirMount(wshost.CacheDir, eg.DefaultCacheDirectory()).
		WithDirMount(wshost.CacheDir, wshost.CacheDir).
		WithDirMount(wshost.WorkspaceDir, eg.DefaultWorkspaceDirectory()).
		WithDirMount(wshost.WorkspaceDir, wshost.WorkspaceDir).
		WithDirMount(wshost.WorkingDir, eg.DefaultWorkingDirectory()).
		WithDirMount(wshost.WorkingDir, wshost.WorkingDir)

	mcfg := wazero.NewModuleConfig().WithEnv(
		"SSL_CERT_DIR", DefaultSSLCertDir,
	).WithEnv(
		"CI", envx.String("true", "CI"),
	).WithEnv(
		eg.EnvComputeRunID, runid,
	).WithEnv(
		eg.EnvComputeAccountID, envx.String(uuid.Nil.String(), eg.EnvComputeAccountID),
	).WithEnv(
		eg.EnvComputeModuleNestedLevel, strconv.Itoa(envx.Int(0, eg.EnvComputeModuleNestedLevel)),
	).WithEnv(
		eg.EnvComputeLoggingVerbosity, envx.String("-1", eg.EnvComputeLoggingVerbosity),
	).WithEnv(
		"PATH", os.Getenv("PATH"),
	).WithEnv(
		"TERM", envx.String("", "TERM"),
	).WithEnv(
		"PWD", eg.DefaultWorkingDirectory(),
	).WithStdin(
		inpr,
	).WithStderr(
		errpw,
	).WithStdout(
		outpw,
	).WithFSConfig(
		wazerofs,
	).WithSysNanotime().WithSysWalltime().WithRandSource(rand.Reader)

	environ := errorsx.Zero(envx.FromPath(eg.DefaultMountRoot(eg.RuntimeDirectory, eg.EnvironFile)))
	// envx.Debug(t.environ...)
	// envx.Debug(environ...)
	mcfg = wasix.Environ(mcfg, t.environ...)
	mcfg = wasix.Environ(mcfg, environ...)

	wasienv, err := wasi_snapshot_preview1.NewBuilder(runtime).Instantiate(ctx)
	if err != nil {
		return errorsx.Wrap(err, "unable to create wasi runtime")
	}
	defer wasienv.Close(ctx)

	wasinet, err := ffiwasinet.Wazero(
		runtime,
		wnetruntime.OptionFSPrefixes(
			wnetruntime.FSPrefix{Host: wshost.RuntimeDir, Guest: eg.DefaultRuntimeDirectory()},
			wnetruntime.FSPrefix{Host: wshost.CacheDir, Guest: eg.DefaultCacheDirectory()},
			wnetruntime.FSPrefix{Host: wshost.WorkingDir, Guest: eg.DefaultWorkingDirectory()},
		),
	).Instantiate(ctx)
	if err != nil {
		return errorsx.Wrap(err, "failed to setup wasinet")
	}
	defer wasinet.Close(ctx)

	if tracedebug {
		wasidebug.Host(wasinet)
	}

	hostenv, err := rtb(t, runtime.NewHostModuleBuilder("env")).
		Instantiate(ctx)
	if err != nil {
		return errorsx.Wrap(err, "failed to setup host environment")
	}
	defer hostenv.Close(ctx)

	if tracedebug {
		wasidebug.Host(hostenv)
	}

	wasi, err := os.ReadFile(path)
	if err != nil {
		return errorsx.Wrap(err, "unable to read module")
	}

	c, err := runtime.CompileModule(ctx, wasi)
	if err != nil {
		return errorsx.Wrap(err, "unable to compile module")
	}
	defer c.Close(ctx)

	// wasidebug.Module(c)

	debugx.Println("interp initiated", path)
	defer debugx.Println("interp completed", path)
	m, err := runtime.InstantiateModule(ctx, c, mcfg.WithName(path))
	if err != nil {
		return errorsx.Wrap(err, "unable to run module")
	}
	defer m.Close(ctx)

	return nil
}
