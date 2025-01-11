package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/runtimex"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiwasinet"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"go.uber.org/automaxprocs/maxprocs"
)

type module struct {
	Dir        string `name:"directory" help:"root directory of the repository" default:"${vars_git_directory}"`
	ModuleDir  string `name:"moduledir" help:"deprecated removed once infrastructure is updated" hidden:"true" default:"${vars_workload_directory}"`
	RuntimeDir string `name:"runtimedir" help:"runtime directory" hidden:"true" default:"${vars_eg_runtime_directory}"`
	Module     string `arg:"" help:"name of the module to run"`
}

func (t module) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		ws    workspaces.Context
		aid   = envx.String(uuid.Nil.String(), eg.EnvComputeAccountID)
		uid   = envx.String(uuid.Nil.String(), eg.EnvComputeRunID)
		descr = envx.String("", eg.EnvComputeVCS)
		cc    grpc.ClientConnInterface
	)

	// ensure when we run modules our umask is set to allow git clones to work properly
	runtimex.Umask(0002)

	if ws, err = workspaces.FromEnv(gctx.Context, t.Dir, t.Module); err != nil {
		return err
	}

	if err = gitx.AutomaticCredentialRefresh(gctx.Context, tlsc.DefaultClient(), t.RuntimeDir, envx.String("", gitx.EnvAuthEGAccessToken)); err != nil {
		return err
	}

	if mlevel := envx.Int(0, eg.EnvComputeModuleNestedLevel); mlevel == 0 || envx.Boolean(false, eg.EnvComputeRootModule) {
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

		log.Println("---------------------------- ROOT MODULE INITIATED ----------------------------")
		log.Println("account", aid)
		log.Println("run id", uid)
		log.Println("repository", descr)
		log.Println("number of cores", runtime.GOMAXPROCS(-1))
		log.Println("ram available", bytesx.Unit(vmemlimit))
		log.Println("logging level", gctx.Verbosity)
		// fsx.PrintDir(os.DirFS(t.RuntimeDir))
		defer log.Println("---------------------------- ROOT MODULE COMPLETED ----------------------------")

		cspath := filepath.Join(t.RuntimeDir, "control.socket")
		if control, err = net.Listen("unix", cspath); err != nil {
			return errorsx.Wrap(err, "unable to create control.socket")
		}
		srv := grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()), // this is a local socket
		)

		if db, err = sql.Open("duckdb", filepath.Join(t.RuntimeDir, "analytics.db")); err != nil {
			return errorsx.Wrap(err, "unable to create analytics.db")
		}
		defer db.Close()
		if err = events.PrepareDB(gctx.Context, db); err != nil {
			return errorsx.Wrap(err, "unable to prepare analytics.db")
		}
		// periodic sampling of system metrics
		go runners.BackgroundSystemLoad(gctx.Context, db)
		// final sample
		defer func() {
			errorsx.Log(runners.SampleSystemLoad(gctx.Context, db))
		}()

		events.NewServiceDispatch(db).Bind(srv)
		ragent := runners.NewRunner(
			gctx.Context,
			ws,
			uid,
			runners.AgentOptionEnv(eg.EnvComputeTLSInsecure, strconv.FormatBool(tlsc.Insecure)),
			runners.AgentOptionVolumes(
				runners.AgentMountReadWrite("/root", "/root"),
				runners.AgentMountReadWrite(eg.DefaultMountRoot(eg.CacheDirectory), eg.DefaultMountRoot(eg.CacheDirectory)),
				runners.AgentMountReadWrite(eg.DefaultMountRoot(eg.RuntimeDirectory), eg.DefaultMountRoot(eg.RuntimeDirectory)),
				runners.AgentMountReadWrite(eg.DefaultMountRoot(eg.TempDirectory), eg.DefaultMountRoot(eg.TempDirectory)),
				runners.AgentMountReadWrite("/var/lib/containers", "/var/lib/containers"),
			),
			runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(eg.DefaultMountRoot("egbin")))),
			runners.AgentOptionCommandLine("--userns", "host"),       // properly map host user into containers.
			runners.AgentOptionCommandLine("--cap-add", "NET_ADMIN"), // required for loopback device creation inside the container
			runners.AgentOptionCommandLine("--cap-add", "SYS_ADMIN"), // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
			runners.AgentOptionCommandLine("--device", "/dev/fuse"),  // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
			runners.AgentOptionCommandLine("--network", "host"),      // ipv4 group bullshit. pretty sure its a podman 4 issue that was resolved in podman 5. this is 'safe' to do because we are already in a container.
			runners.AgentOptionCommandLine("--pids-limit", "-1"),     // more bullshit. without this we get "Error: OCI runtime error: crun: the requested cgroup controller `pids` is not available"
		)

		c8s.NewServiceProxy(
			log.Default(),
			ws,
			c8s.ServiceProxyOptionCommandEnviron(
				errorsx.Zero(
					envx.Build().FromEnv(
						"PATH",
						"TERM",
						"COLORTERM",
						"LANG",
						"CI",
						"EG_CI",
						eg.EnvComputeBin,
						eg.EnvComputeContainerExec,
						eg.EnvComputeRunID,
						eg.EnvComputeAccountID,
					).Environ(),
				)...,
			),
			c8s.ServiceProxyOptionContainerOptions(
				ragent.Options()...,
			),
		).Bind(srv)

		go func() {
			errorsx.Log(errorsx.Wrap(srv.Serve(control), "unable to create control socket"))
		}()
		defer srv.GracefulStop()
	} else {
		debugx.Printf("---------------------------- MODULE INITIATED %d ----------------------------\n", mlevel)
		// env.Debug(os.Environ()...)
		debugx.Println("account", aid)
		debugx.Println("run id", uid)
		debugx.Println("repository", descr)
		debugx.Println("number of cores", runtime.GOMAXPROCS(-1))
		debugx.Println("logging level", gctx.Verbosity)
		defer debugx.Printf("---------------------------- MODULE COMPLETED %d ----------------------------\n", mlevel)
	}

	if cc, err = daemons.AutoRunnerClient(gctx, ws, uid, runners.AgentOptionAutoEGBin()); err != nil {
		return err
	}

	return interp.Remote(
		gctx.Context,
		aid,
		uid,
		cc,
		t.Dir,
		t.Module,
		interp.OptionRuntimeDir(t.RuntimeDir),
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

	mpath := ws.Temporary("test.wasm")
	log.Println("wasipath", ws.TemporaryDir, mpath)
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
