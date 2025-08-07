package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/runtimex"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiwasinet"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"go.uber.org/automaxprocs/maxprocs"
)

type module struct {
	Dir        string `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	ModuleDir  string `name:"moduledir" help:"deprecated removed once infrastructure is updated" hidden:"true" default:"${vars_workload_directory}"`
	RuntimeDir string `name:"runtimedir" help:"runtime directory" hidden:"true" default:"${vars_eg_runtime_directory}"`
	Module     string `arg:"" help:"name of the module to run"`
}

func (t module) mounthack(ctx context.Context, runid string, ws workspaces.Context) (err error) {
	var (
		binname = "bindfs"
		mbin    string
	)

	if mbin, err = exec.LookPath(binname); err != nil {
		return errorsx.Wrap(err, "unable to locate bindfs, failing")
	}

	remap := func(from, to string) error {
		mcmd := exec.CommandContext(ctx, mbin, "--map=root/egd:@root/@egd", from, to)
		if err = execx.MaybeRun(mcmd); err != nil {
			return errorsx.Wrapf(err, "unable to run bindfs: %s", from)
		}

		return nil
	}

	err = fsx.MkDirs(
		0770,
		eg.DefaultWorkingDirectory(),
		eg.DefaultCacheDirectory(),
		eg.DefaultWorkloadDirectory(),
	)
	if err != nil {
		return err
	}

	err = errors.Join(
		remap(eg.DefaultMountRoot(eg.WorkingDirectory), eg.DefaultWorkingDirectory()),
		remap(eg.DefaultMountRoot(eg.CacheDirectory), eg.DefaultCacheDirectory()),
		remap(eg.DefaultMountRoot(eg.WorkloadDirectory), eg.DefaultWorkloadDirectory()),
	)
	if err != nil {
		return err
	}

	// HACK: gpg no longer obeys GNUPGHOME for the root user.
	if err = errorsx.Compact(fsx.MkDirs(0700, "/run/user/0/gnupg"), os.Symlink("/eg.mnt/.gnupg", "/run/user/0/gnupg/d.3uqafziaitu1mwy9asijecpi")); err != nil {
		return err
	}

	if err = fsx.Wait(ctx, 3*time.Second, ws.Root); err != nil {
		return errorsx.Wrapf(err, "expected working directory (%s) did not appear, this is a known issue", ws.Root)
	}

	return nil
}

func (t module) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		ws      workspaces.Context
		aid     = envx.String(uuid.Nil.String(), eg.EnvComputeAccountID)
		uid     = envx.String(uuid.Nil.String(), eg.EnvComputeRunID)
		descr   = envx.String("", eg.EnvComputeVCS)
		hostnet = envx.Toggle(runners.AgentOptionCommandLine("--network", "host"), runners.AgentOptionNoop, eg.EnvExperimentalDisableHostNetwork) // ipv4 group bullshit. pretty sure its a podman 4 issue that was resolved in podman 5. this is 'safe' to do because we are already in a container.
		cc      grpc.ClientConnInterface
		cmdenv  []string
	)

	// ensure when we run modules our umask is set to allow git clones to work properly
	runtimex.Umask(0002)

	if ws, err = workspaces.FromEnv(gctx.Context, t.Dir, t.Module); err != nil {
		return err
	}

	debugx.Println(spew.Sdump(ws))

	cmdenvb := envx.Build().FromEnv(
		"PATH",
		"TERM",
		"COLORTERM",
		"LANG",
		"CI",
		eg.EnvComputeBin,
		eg.EnvComputeRunID,
		eg.EnvComputeAccountID,
	).Var(
		eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory(),
	).Var(
		eg.EnvComputeCacheDirectory, envx.String(eg.DefaultCacheDirectory(), eg.EnvComputeCacheDirectory, "CACHE_DIRECTORY"),
	).Var(
		eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory(),
	).Var(
		eg.EnvComputeWorkloadDirectory, eg.DefaultWorkloadDirectory(),
	).Var(
		"PAGER", "cat", // no paging in this environmenet.
	)

	if mlevel := envx.Int(0, eg.EnvComputeModuleNestedLevel); mlevel == 0 {
		var (
			control   net.Listener
			db        *sql.DB
			vmemlimit int64
		)

		// automatically detect the correct number of max procs for the module
		if _, err = maxprocs.Set(maxprocs.Logger(log.Printf)); err != nil {
			return errorsx.Wrap(err, "unable to set cpu limits")
		}

		if vmemlimit, err = memlimit.SetGoMemLimitWithOpts(memlimit.WithProvider(memlimit.FromCgroup)); err != nil {
			return errorsx.Wrap(err, "unable to set max limits")
		}

		debugx.Println("---------------------------- ROOT MODULE INITIATED ----------------------------")
		debugx.Println("module pid", os.Getpid())
		debugx.Println("account", aid)
		debugx.Println("run id", uid)
		debugx.Println("repository", descr)
		debugx.Println("number of cores (GOMAXPROCS - inaccurate)", runtime.GOMAXPROCS(-1))
		debugx.Println("ram available", bytesx.Unit(vmemlimit))
		debugx.Println("logging level", gctx.Verbosity)
		// fsx.PrintDir(os.DirFS(t.RuntimeDir))
		defer debugx.Println("---------------------------- ROOT MODULE COMPLETED ----------------------------")

		cspath := filepath.Join(t.RuntimeDir, eg.SocketControl)
		if control, err = net.Listen("unix", cspath); err != nil {
			return errorsx.Wrapf(err, "unable to create %s", cspath)
		}
		defer control.Close()

		if db, err = sql.Open("duckdb", filepath.Join(t.RuntimeDir, "analytics.db")); err != nil {
			return errorsx.Wrap(err, "unable to create analytics.db")
		}
		defer db.Close()

		if err = events.PrepareDB(gctx.Context, db); err != nil {
			return errorsx.Wrap(err, "unable to prepare analytics.db")
		}

		cmdenvb = cmdenvb.Var(
			eg.EnvComputeModuleSocket, eg.DefaultMountRoot(eg.RuntimeDirectory, filepath.Base(cspath)),
		).FromEnviron(
			os.Environ()...,
		)
		if cmdenv, err = cmdenvb.Environ(); err != nil {
			return err
		}

		// periodic sampling of system metrics
		go runners.BackgroundSystemLoad(gctx.Context, db)

		// final sample
		defer func() {
			fctx, done := context.WithTimeout(context.Background(), 10*time.Second)
			defer done()
			errorsx.Log(runners.SampleSystemLoad(fctx, db))
		}()
		srv := grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()), // this is a local socket
			grpc.ChainUnaryInterceptor(
				podmanx.GrpcClient,
			),
		)
		defer srv.GracefulStop()

		events.NewServiceDispatch(db).Bind(srv)
		execproxy.NewExecProxy(t.Dir, cmdenv).Bind(srv)

		ragent := runners.NewRunner(
			gctx.Context,
			ws,
			uid,
			runners.AgentOptionCommandLine("--env-file", eg.DefaultRuntimeDirectory(eg.EnvironFile)), // required for tty to work correct in local mode.
			runners.AgentOptionEnv(eg.EnvComputeTLSInsecure, strconv.FormatBool(tlsc.Insecure)),
			runners.AgentOptionVolumes(
				runners.AgentMountReadWrite("/root", "/root"),
				runners.AgentMountReadWrite(eg.DefaultMountRoot(), eg.DefaultMountRoot()),
				runners.AgentMountReadWrite("/var/lib/containers", "/var/lib/containers"),
			),
			runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(eg.DefaultMountRoot(eg.BinaryBin)))),
			runners.AgentOptionHostOS(),
			hostnet,
		)

		c8sproxy.NewServiceProxy(
			log.Default(),
			ws,
			c8sproxy.ServiceProxyOptionCommandEnviron(cmdenv...),
			c8sproxy.ServiceProxyOptionContainerOptions(
				ragent.Options()...,
			),
		).Bind(srv)

		go func() {
			errorsx.Log(errorsx.Wrap(srv.Serve(control), "unable to serve control socket"))
		}()

		if err = gitx.AutomaticCredentialRefresh(gctx.Context, tlsc.DefaultClient(), t.RuntimeDir, envx.String("", gitx.EnvAuthEGAccessToken)); err != nil {
			return err
		}
	} else {
		var (
			control net.Listener
		)

		mspath := filepath.Join(t.RuntimeDir, eg.SocketModule())
		if control, err = net.Listen("unix", mspath); err != nil {
			return errorsx.Wrapf(err, "unable to create %s", mspath)
		}
		defer control.Close()

		debugx.Printf("---------------------------- MODULE INITIATED %d ----------------------------\n", mlevel)
		debugx.Fn(func() { envx.Debug(os.Environ()...) })
		debugx.Println("module pid", os.Getpid())
		debugx.Println("account", aid)
		debugx.Println("run id", uid)
		debugx.Println("repository", descr)
		debugx.Println("number of cores", runtime.GOMAXPROCS(-1))
		debugx.Println("logging level", gctx.Verbosity)
		debugx.Println("module pid", os.Getpid())
		debugx.Println("mspath", mspath)
		defer debugx.Printf("---------------------------- MODULE COMPLETED %d ----------------------------\n", mlevel)

		srv := grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()), // this is a local socket
			grpc.ChainUnaryInterceptor(
				podmanx.GrpcClient,
			),
		)
		defer srv.GracefulStop()

		cmdenvb = cmdenvb.Var(
			eg.EnvComputeModuleSocket, eg.DefaultMountRoot(eg.RuntimeDirectory, filepath.Base(mspath)),
		).FromEnviron(
			os.Environ()...,
		)

		if cmdenv, err = cmdenvb.Environ(); err != nil {
			return err
		}

		execproxy.NewExecProxy(t.Dir, cmdenv).Bind(srv)
		go func() {
			errorsx.Log(errorsx.Wrap(srv.Serve(control), "unable to serve control socket"))
		}()
	}

	// IMPORTANT: duckdb does not play well with bindfs mounting the folders before
	// creating extensions/tables, it would nuke the working directory. it *mostly* worked once we moved this mount
	// call after duckdb. we're leaving the call here for now but it shouldn't matter.
	// and we're keen to remove it.
	if err = t.mounthack(gctx.Context, uid, ws); err != nil {
		return errorsx.Wrap(err, "unable to mount with correct permissions - this is a transient error that happens occassional likely due to bindfs/podman bugs")
	}

	if cc, err = daemons.AutoRunnerClient(gctx, ws, uid, runners.AgentOptionAutoEGBin()); err != nil {
		return err
	}

	return interp.Remote(
		gctx.Context,
		ws,
		aid,
		uid,
		cc,
		t.Module,
		interp.OptionEnviron(cmdenv...),
	)
}

type wasiCmd struct {
	Dir       string `name:"directory" help:"root directory of the repository" default:"${vars_git_directory}"`
	Module    string `arg:"" help:"name of the module to run"`
	ModuleDir string `name:"moduledir" help:"deprecated removed once infrastructure is updated" hidden:"true" default:"${vars_workload_directory}"`
}

func (t wasiCmd) Run(gctx *cmdopts.Global) (err error) {
	var (
		ws workspaces.Context
	)

	ctx, done := context.WithCancel(gctx.Context)
	defer done()

	if ws, err = workspaces.FromEnv(gctx.Context, t.Dir, t.Module); err != nil {
		return err
	}

	mpath := filepath.Join(ws.RuntimeDir, "test.wasm")
	log.Println("wasipath", ws.RuntimeDir, mpath)
	if err = compile.Run(ctx, t.Dir, t.Module, mpath); err != nil {
		return err
	}
	runtime := wazero.NewRuntimeWithConfig(
		ctx,
		wazero.NewRuntimeConfigInterpreter(),
	)

	mcfg := wazero.NewModuleConfig().WithStdin(
		os.Stdin,
	).WithStderr(
		os.Stderr,
	).WithStdout(
		os.Stdout,
	).WithFSConfig(
		wazero.NewFSConfig().
			WithDirMount("/etc/resolv.conf", "/etc/resolv.conf"),
	).WithSysNanotime().WithSysWalltime().WithRandSource(rand.Reader)

	environ := errorsx.Zero(envx.Build().FromEnviron(os.Environ()...).Environ())
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

	hostenv, err := runtime.
		NewHostModuleBuilder("env").
		Instantiate(ctx)
	if err != nil {
		return err
	}
	defer hostenv.Close(ctx)

	log.Println("interp initiated", mpath)
	defer log.Println("interp completed", mpath)
	wasi, err := os.ReadFile(mpath)
	if err != nil {
		return err
	}

	c, err := runtime.CompileModule(ctx, wasi)
	if err != nil {
		return err
	}
	defer c.Close(ctx)

	// wasidebug.Module(c)

	m, err := runtime.InstantiateModule(ctx, c, mcfg.WithName(mpath))
	if err != nil {
		return err
	}
	defer m.Close(ctx)

	return nil
}
