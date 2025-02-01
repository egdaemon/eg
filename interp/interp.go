package interp

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
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

type runtimefn func(r runner, cmdenv []string, host wazero.HostModuleBuilder) wazero.HostModuleBuilder

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

	runtimeenv := func(r runner, cmdenv []string, host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
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
			cmd.Env = append(cmdenv, cmd.Env...)
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
	initonce   *sync.Once
}

func (t runner) perform(ctx context.Context, runid, path string, rtb runtimefn) (err error) {
	tracedebug := envx.Boolean(false, eg.EnvLogsTrace)
	hostcachedir := filepath.Join(t.runtimedir, "cache")
	if err = fsx.MkDirs(0770, hostcachedir); err != nil {
		return errorsx.Wrap(err, "unable to ensure host cache directory")
	}

	debugx.Println("wazero cache", wasix.WazCacheDir(t.runtimedir))
	// fsx.PrintFS(os.DirFS(wasix.WazCacheDir(t.runtimedir)))

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

	cmdenv, err := envx.Build().FromEnviron(
		os.Environ()...,
	).FromEnv(
		"CI",
		eg.EnvComputeRunID,
		eg.EnvComputeAccountID,
	).Var(
		eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory(),
	).Var(
		eg.EnvComputeCacheDirectory, envx.String(eg.DefaultCacheDirectory(), eg.EnvComputeCacheDirectory, "CACHE_DIRECTORY"),
	).Var(
		eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory(),
	).Var(
		"RUNTIME_DIRECTORY", eg.DefaultRuntimeDirectory(),
	).Environ()
	if err != nil {
		return err
	}

	debugx.Println("cache dir", hostcachedir, "->", eg.DefaultCacheDirectory())
	debugx.Println("runtime dir", t.runtimedir, "->", eg.DefaultMountRoot(eg.RuntimeDirectory))
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
		eg.EnvComputeCacheDirectory, envx.String(eg.DefaultCacheDirectory(), eg.EnvComputeCacheDirectory, "CACHE_DIRECTORY"),
	).WithEnv(
		eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory(),
	).WithEnv(
		"RUNTIME_DIRECTORY", eg.DefaultRuntimeDirectory(),
	).WithEnv(
		"PATH", os.Getenv("PATH"),
	).WithEnv(
		"TERM", envx.String("", "TERM"),
	).WithEnv(
		"PWD", eg.DefaultWorkingDirectory(),
	).WithStdin(
		os.Stdin,
	).WithStderr(
		os.Stderr,
	).WithStdout(
		os.Stdout,
	).WithFSConfig(
		wazero.NewFSConfig().
			WithDirMount(os.TempDir(), os.TempDir()).
			WithDirMount(t.runtimedir, eg.DefaultRuntimeDirectory()).
			WithDirMount(hostcachedir, eg.DefaultCacheDirectory()).
			WithDirMount(t.root, eg.DefaultWorkingDirectory()). // ensure we mount the working directory so pwd works correctly.
			WithDirMount(t.runtimedir, eg.DefaultMountRoot(eg.RuntimeDirectory)).
			WithDirMount(hostcachedir, eg.DefaultMountRoot(eg.CacheDirectory)).
			WithDirMount(t.root, eg.DefaultMountRoot(eg.WorkingDirectory)), // ensure we mount the working directory so pwd works correctly.
	).WithSysNanotime().WithSysWalltime().WithRandSource(rand.Reader)

	environ := errorsx.Zero(envx.FromPath(eg.DefaultMountRoot(eg.RuntimeDirectory, "environ.env")))
	// envx.Debug(environ...)
	mcfg = wasix.Environ(mcfg, environ...)

	wasienv, err := wasi_snapshot_preview1.NewBuilder(runtime).Instantiate(ctx)
	if err != nil {
		return err
	}
	defer wasienv.Close(ctx)

	wasinet, err := ffiwasinet.Wazero(runtime).Instantiate(ctx)
	if err != nil {
		return err
	}
	defer wasinet.Close(ctx)

	if tracedebug {
		wasidebug.Host(wasinet)
	}

	hostenv, err := rtb(t, cmdenv, runtime.NewHostModuleBuilder("env")).
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
