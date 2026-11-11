package interp

import (
	"context"
	"crypto/rand"
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiegvm"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiexec"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigit"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigraph"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiwasinet"
	"github.com/egdaemon/eg/interp/runtime/wasi/wasimod"
	"github.com/egdaemon/eg/interp/wasidebug"
	"github.com/egdaemon/eg/workspaces"
	"github.com/egdaemon/wasinet/wasinet/wnetruntime"
	"github.com/gofrs/uuid/v5"
	"github.com/opencontainers/go-digest"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental"
	"github.com/tetratelabs/wazero/experimental/logging"
	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/experimental/sysfs"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/grpc"
)

type Option func(*runner)

func OptionEnviron(s ...string) Option {
	return func(r *runner) {
		r.environ = s
	}
}

// OptionHTTPClient is the client egiso images are fetched with.
func OptionHTTPClient(c *http.Client) Option {
	return func(r *runner) {
		r.client = c
	}
}

// OptionRoot mounts root at / in place of the empty read-only root.
func OptionRoot(root experimentalsys.FS) Option {
	return func(r *runner) {
		r.root = root
	}
}

// OptionRepoCache is the root directory repositories are mirrored in so clones are incremental, blank disables caching.
func OptionRepoCache(root string) Option {
	return func(r *runner) {
		r.repocache = root
	}
}

type runtimefn func(r runner, host wazero.HostModuleBuilder) wazero.HostModuleBuilder

// moduleenviron is the environment the runner provides to the module, read from its runtime directory.
func moduleenviron(wshost workspaces.Context) []string {
	// in a container the runtime directory of the workspace is mounted at the default mount root. baremetal runs
	// on the host, where the default mount root is that of the workload that launched it, if it exists at all.
	return errorsx.Zero(envx.FromPath(filepath.Join(wshost.RuntimeDir, eg.EnvironFile)))
}

// Remote uses the api to implement particular actions like building and running containers.
// may not be necessary.
func Remote(ctx context.Context, wshost workspaces.Context, aid string, runid string, svc grpc.ClientConnInterface, module string, options ...Option) error {
	var (
		r = langx.Clone(runner{
			initonce: &sync.Once{},
			root:     &sysfs.AdaptFS{FS: embed.FS{}},
			client:   http.DefaultClient,
		}, options...)
	)

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
			NewFunctionBuilder().WithFunc(ffiegvm.Run(func(ctx context.Context, location string, toc string, modulepath string, environ ...string) (err error) {
			var (
				pinned digest.Digest
			)

			if pinned, err = digest.Parse(toc); err != nil {
				return errorsx.Wrapf(err, "invalid table of contents digest for %s", location)
			}

			path := fsx.LocateFirst(
				filepath.Join(wshost.Root, wshost.BuildDir, modulepath),
				eg.DefaultMountRoot(eg.RuntimeDirectory, modulepath),
			)
			return Cell(ctx, wshost, aid, runid, svc, r.client, path, location, pinned, append(slices.Clone(r.environ), environ...)...)
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegvm.Run").
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
			ffigit.CloneV2(fsx.JoinIfPresent(r.repocache, aid), wshost.WorkingDir, wshost.RuntimeDir),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.CloneV2").
			NewFunctionBuilder().WithFunc(
			ffigit.CloneV3(fsx.JoinIfPresent(r.repocache, aid), wshost.RuntimeDir, workspaces.Remap(wshost)),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.CloneV3").
			NewFunctionBuilder().WithFunc(
			ffigit.Bearer(wshost.RuntimeDir),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Bearer").
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

// mounts the workspace directories into the guest under root.
// we map twice so that baremetal can work. shell commands are run on the host itself so we need the host path.
// but inside wazero we need to be able to access the standard paths.
// the call decide which paths the environment variables rsolve to.
// we also need to ensure we mount the working directory so pwd works correctly.
// the root lets guests that resolve paths from / (wasi-libc programs such as coreutils) reach the nested mounts,
// and everything outside of them is missing or read-only.
func mounts(wshost workspaces.Context, root experimentalsys.FS) wazero.FSConfig {
	return wazero.NewFSConfig().(sysfs.FSConfig).
		WithSysFSMount(root, "/").
		WithDirMount(os.TempDir(), os.TempDir()).
		WithDirMount(wshost.Root, eg.DefaultRootDirectory()).
		WithDirMount(wshost.Root, wshost.Root).
		WithDirMount(wshost.RuntimeDir, eg.DefaultRuntimeDirectory()).
		WithDirMount(wshost.RuntimeDir, wshost.RuntimeDir).
		WithDirMount(wshost.CacheDir, eg.DefaultCacheDirectory()).
		WithDirMount(wshost.CacheDir, wshost.CacheDir).
		WithDirMount(wshost.WorkspaceDir, eg.DefaultWorkspaceDirectory()).
		WithDirMount(wshost.WorkspaceDir, wshost.WorkspaceDir).
		WithDirMount(wshost.WorkingDir, eg.DefaultWorkingDirectory()).
		WithDirMount(wshost.WorkingDir, wshost.WorkingDir)
}

type runner struct {
	environ  []string
	initonce *sync.Once
	root     experimentalsys.FS
	client   *http.Client
	// repocache is the root directory repositories are mirrored in, blank disables caching.
	repocache string
}

func (t runner) modulenv(mcfg wazero.ModuleConfig, wshost workspaces.Context) wazero.ModuleConfig {
	hostenviron := moduleenviron(wshost)
	// envx.Debug(t.environ...)
	// envx.Debug(hostenviron...)
	mcfg = wasix.Environ(mcfg, hostenviron...)
	mcfg = wasix.Environ(mcfg, t.environ...)
	return mcfg
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
		// closing modules when their context is done is what stops a coreutils call at its deadline.
		wazero.NewRuntimeConfig().WithCompilationCache(cache).WithCloseOnContextDone(true),
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

	debugx.Println("workload dir", wshost.Root, "->", eg.DefaultRootDirectory())
	debugx.Println("cache dir", wshost.CacheDir, "->", eg.DefaultCacheDirectory())
	debugx.Println("runtime dir", wshost.RuntimeDir, "->", eg.DefaultRuntimeDirectory())
	debugx.Println("working dir", wshost.WorkingDir, "->", eg.DefaultWorkingDirectory())
	debugx.Println("workspace dir", wshost.WorkspaceDir, "->", eg.DefaultWorkspaceDirectory())
	debugx.Println("wazero cache", wshost.CacheDirWazero)
	debugx.Println("system tls certs", hostsslcerts)
	debugx.Println("------------------------------------------------------")

	wazerofs := mounts(wshost, t.root).WithDirMount(hostsslcerts, DefaultSSLCertDir)

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
		eg.EnvComputeMemoryLimit, envx.String("", eg.EnvComputeMemoryLimit),
	).WithEnv(
		eg.EnvComputeCoresLimit, envx.String("", eg.EnvComputeCoresLimit),
	).WithEnv(
		eg.EnvComputeGuestMigrationsEnabled, strconv.FormatBool(envx.Boolean(true, eg.EnvComputeGuestMigrationsEnabled)),
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
	mcfg = t.modulenv(mcfg, wshost)

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

	wasimods, err := wasimod.Load(ctx, runtime, wasimod.Directory(), mcfg)
	if err != nil {
		return errorsx.Wrap(err, "unable to load the wasi modules")
	}
	defer wasimods.Close(ctx)

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
