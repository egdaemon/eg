package compute

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/runtimex"
	"github.com/egdaemon/eg/interp"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type baremetal struct {
	Dir        string `name:"directory" help:"root directory of the repository" default:"${vars_git_directory}"`
	RuntimeDir string `name:"runtimedir" help:"runtime directory" hidden:"true" default:"${vars_eg_runtime_directory}"`
	Module     string `arg:"" help:"name of the module to run"`
}

func (t baremetal) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
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
		"PAGER", "cat", // no paging in this environmenet.
	)

	if mlevel := envx.Int(0, eg.EnvComputeModuleNestedLevel); mlevel == 0 {
		var (
			control   net.Listener
			db        *sql.DB
			vmemlimit int64
		)

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
		// env.Debug(os.Environ()...)
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
		interp.OptionEnviron(cmdenv...),
	)
}
