package interp

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/runtime/wasi/fficoverage"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiegcontainer"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiexec"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigit"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigraph"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffimetric"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiwasinet"
	"github.com/egdaemon/eg/interp/wasidebug"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/wasinet/wasinet/wnetruntime"
	"github.com/gofrs/uuid"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/experimental/logging"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/grpc"
)

type Option func(*runner)

// OptionRuntimeDir
func OptionRuntimeDir(s string) Option {
	return func(r *runner) {
		r.runtimedir = s
	}
}

func OptionEnviron(s ...string) Option {
	return func(r *runner) {
		r.environ = s
	}
}

type runtimefn func(r runner, host wazero.HostModuleBuilder) wazero.HostModuleBuilder

// Remote uses the api to implement particular actions like building and running containers.
// may not be necessary.
func Remote(ctx context.Context, aid string, runid string, svc grpc.ClientConnInterface, dir string, module string, options ...Option) error {
	var (
		r = runner{
			root:       dir,
			runtimedir: runners.DefaultRunnerDirectory(runid),
			initonce:   &sync.Once{},
		}
	)
	for _, opt := range options {
		opt(&r)
	}

	containers := c8s.NewProxyClient(svc)
	evtclient := events.NewEventsClient(svc)

	runtimeenv := func(r runner, host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
		return host.
			NewFunctionBuilder().WithFunc(ffigraph.Trace(evtclient)).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Trace").
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
				"--volume", fmt.Sprintf("%s:%s:ro", filepath.Join(r.runtimedir, modulepath), eg.DefaultMountRoot(eg.ModuleBin)),
			)

			_, err = containers.Module(ctx, &c8s.ModuleRequest{
				Image:   name,
				Name:    cname,
				Mdir:    r.runtimedir,
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
				cmd.Dir = filepath.Join(r.root, cmd.Dir)
			}
			cmd.Env = append(r.environ, cmd.Env...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			return cmd
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiexec.Command").
			NewFunctionBuilder().WithFunc(
			ffigit.Commitish(r.root),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Commitish").
			NewFunctionBuilder().WithFunc(
			ffigit.CloneV1(r.root),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Clone").
			NewFunctionBuilder().WithFunc(
			ffigit.CloneV2(r.root, r.runtimedir),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.CloneV2").
			NewFunctionBuilder().WithFunc(
			ffimetric.Metric(evtclient),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/metrics.Record").
			NewFunctionBuilder().WithFunc(
			fficoverage.Report(evtclient),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/coverage.Report")
	}

	return r.perform(ctx, runid, module, runtimeenv)
}

type runner struct {
	root       string
	runtimedir string
	environ    []string
	initonce   *sync.Once
}

func (t runner) perform(ctx context.Context, runid, path string, rtb runtimefn) (err error) {
	defer func(before bool) {
		// strictly speaking stdin should remain blocking at all times but using before
		if nonBlocking(os.Stdin.Fd()) == before {
			debugx.Println("---------------------------------------------- stdin was munged ----------------------------------------------")
		}
	}(nonBlocking(os.Stdin.Fd()))
	tracedebug := envx.Boolean(false, eg.EnvLogsTrace)

	cache, err := wazero.NewCompilationCacheWithDir(wasix.WazCacheDir(t.runtimedir))
	if err != nil {
		return err
	}
	defer cache.Close(ctx)

	if tracedebug {
		ctx = experimental.WithFunctionListenerFactory(ctx,
			logging.NewHostLoggingListenerFactory(os.Stderr, logging.LogScopeAll))
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
	// inpr, inpw, err := os.Pipe()
	// if err != nil {
	// 	return errorsx.Wrap(err, "failed to open pipe for stdin")
	// }
	// defer inpr.Close()
	// defer inpw.Close()
	// go func() {
	// 	_, _err := io.Copy(inpw, os.Stdin)
	// 	debugx.Println(errorsx.Wrap(_err, "failed copying stdin"))
	// }()

	outpr, outpw := io.Pipe()
	defer outpr.Close()
	defer outpw.CloseWithError(io.EOF)
	go func() {
		_, _err := io.Copy(os.Stdout, outpr)
		_err = errorsx.Ignore(_err, io.ErrClosedPipe)
		errorsx.Log(errorsx.Wrapf(_err, "failed copying to stdout: %T", _err))
		outpw.CloseWithError(_err)
	}()

	// outpr, outpw, err := os.Pipe()
	// if err != nil {
	// 	return errorsx.Wrap(err, "failed to open pipe for stdout")
	// }
	// defer outpr.Close()
	// defer outpw.Close()
	// go func() {
	// 	_, _err := io.Copy(os.Stdout, outpr)
	// 	debugx.Println(errorsx.Wrap(_err, "failed copying to stdout"))
	// }()

	errpr, errpw := io.Pipe()
	defer errpr.Close()
	defer errpw.CloseWithError(io.EOF)
	go func() {
		_, _err := io.Copy(os.Stderr, errpr)
		_err = errorsx.Ignore(_err, io.ErrClosedPipe)
		errorsx.Log(errorsx.Wrap(_err, "failed copying to stderr"))
		errpw.CloseWithError(_err)
	}()

	debugx.Println("cache dir", eg.DefaultCacheDirectory(), "->", eg.DefaultCacheDirectory())
	debugx.Println("runtime dir", t.runtimedir, "->", eg.DefaultMountRoot(eg.RuntimeDirectory))
	debugx.Println("wazero cache", wasix.WazCacheDir(t.runtimedir))
	mcfg := wazero.NewModuleConfig().WithEnv(
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
		eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory(),
	).WithEnv(
		eg.EnvComputeCacheDirectory, eg.DefaultCacheDirectory(),
	).WithEnv(
		eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory(),
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
		wazero.NewFSConfig().
			WithDirMount(os.TempDir(), os.TempDir()).
			WithDirMount(t.runtimedir, eg.DefaultRuntimeDirectory()).
			WithDirMount(eg.DefaultCacheDirectory(), eg.DefaultCacheDirectory()).
			WithDirMount(t.root, eg.DefaultWorkingDirectory()). // ensure we mount the working directory so pwd works correctly.
			WithDirMount(t.runtimedir, eg.DefaultMountRoot(eg.RuntimeDirectory)).
			WithDirMount(eg.DefaultCacheDirectory(), eg.DefaultMountRoot(eg.CacheDirectory)).
			WithDirMount(t.root, eg.DefaultMountRoot(eg.WorkingDirectory)), // ensure we mount the working directory so pwd works correctly.
	).WithSysNanotime().WithSysWalltime().WithRandSource(rand.Reader)

	environ := errorsx.Zero(envx.FromPath(eg.DefaultMountRoot(eg.RuntimeDirectory, eg.EnvironFile)))
	// envx.Debug(t.environ...)
	// envx.Debug(environ...)
	mcfg = wasix.Environ(mcfg, t.environ...)
	mcfg = wasix.Environ(mcfg, environ...)

	wasienv, err := wasi_snapshot_preview1.NewBuilder(runtime).Instantiate(ctx)
	if err != nil {
		return err
	}
	defer wasienv.Close(ctx)

	wasinet, err := ffiwasinet.Wazero(
		runtime,
		wnetruntime.OptionFSPrefixes(
			wnetruntime.FSPrefix{Host: t.runtimedir, Guest: eg.DefaultRuntimeDirectory()},
			wnetruntime.FSPrefix{Host: eg.DefaultCacheDirectory(), Guest: eg.DefaultMountRoot(eg.CacheDirectory)},
			wnetruntime.FSPrefix{Host: t.root, Guest: eg.DefaultMountRoot(eg.WorkingDirectory)},
		),
	).Instantiate(ctx)
	if err != nil {
		return err
	}
	defer wasinet.Close(ctx)

	if tracedebug {
		wasidebug.Host(wasinet)
	}

	hostenv, err := rtb(t, runtime.NewHostModuleBuilder("env")).
		Instantiate(ctx)
	if err != nil {
		return err
	}
	defer hostenv.Close(ctx)

	if tracedebug {
		wasidebug.Host(hostenv)
	}

	debugx.Println("interp initiated", path)
	defer debugx.Println("interp completed", path)
	wasi, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	c, err := runtime.CompileModule(ctx, wasi)
	if err != nil {
		return err
	}
	defer c.Close(ctx)

	// wasidebug.Module(c)

	m, err := runtime.InstantiateModule(ctx, c, mcfg.WithName(path))
	if err != nil {
		return err
	}
	defer m.Close(ctx)

	return nil
}
