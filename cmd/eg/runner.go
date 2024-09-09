package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/interp"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigraph"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"go.uber.org/automaxprocs/maxprocs"
)

type module struct {
	Dir        string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir  string `name:"moduledir" help:"deprecated removed once infrastructure is updated" hidden:"true" default:".eg"`
	RuntimeDir string `name:"runtimedir" help:"runtime directory" hidden:"true" default:"/opt/egruntime/"`
	Module     string `arg:"" help:"name of the module to run"`
}

func (t module) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		ws   workspaces.Context
		aid  = envx.String(uuid.Nil.String(), eg.EnvComputeAccountID)
		uid  = envx.String(uuid.Nil.String(), eg.EnvComputeRunID)
		ebuf = make(chan *ffigraph.EventInfo)
		cc   grpc.ClientConnInterface
	)

	if ws, err = workspaces.FromEnv(gctx.Context, t.Dir, t.Module); err != nil {
		return err
	}

	if err = gitx.AutomaticCredentialRefresh(gctx.Context, tlsc.DefaultClient(), t.RuntimeDir, envx.String("", gitx.EnvAuthEGAccessToken)); err != nil {
		return err
	}

	if envx.Boolean(false, eg.EnvComputeRootModule) {
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
		log.Println("number of cores", runtime.GOMAXPROCS(-1))
		log.Println("ram available", bytesx.Unit(vmemlimit))
		// fsx.PrintDir(os.DirFS("/opt"))
		// fsx.PrintDir(os.DirFS("/opt/egruntime"))
		// fsx.PrintDir(os.DirFS("/opt/eg/workspace"))
		// env.Debug(os.Environ()...)
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
		if err = events.PrepareMetrics(gctx.Context, db); err != nil {
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
			runners.AgentOptionVolumes(
				runners.AgentMountReadWrite("/root", "/root"),
				runners.AgentMountReadWrite("/cache", "/cache"),
				runners.AgentMountReadWrite("/opt/egruntime", "/opt/egruntime"),
				runners.AgentMountReadWrite("/var/lib/containers", "/var/lib/containers"),
				runners.AgentMountReadOnly(errorsx.Must(exec.LookPath("/opt/egbin")), "/opt/egbin"),
			),
			runners.AgentOptionCommandLine("--cap-add", "NET_ADMIN"), // required for loopback device creation inside the container
			runners.AgentOptionCommandLine("--cap-add", "SYS_ADMIN"), // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
			runners.AgentOptionCommandLine("--device", "/dev/fuse"),  // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
			runners.AgentOptionCommandLine("--network", "host"),      // ipv4 group bullshit. pretty sure its a podman 4 issue that was resolved in podman 5. this is 'safe' to do because we are already in a container.
		)

		c8s.NewServiceProxy(
			log.Default(),
			ws,
			c8s.ServiceProxyOptionCommandEnviron(
				errorsx.Zero(
					envx.Build().FromEnv("PATH", "TERM", "COLORTERM", "LANG", "CI", "EG_CI", eg.EnvComputeBin, eg.EnvComputeContainerExec, eg.EnvComputeRunID).Environ(),
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
		log.Println("---------------------------- MODULE INITIATED ----------------------------")
		// env.Debug(os.Environ()...)
		defer log.Println("---------------------------- MODULE COMPLETED ----------------------------")
	}

	if cc, err = daemons.AutoRunnerClient(gctx, ws, uid, runners.AgentOptionAutoEGBin()); err != nil {
		return err
	}

	go func() {
		makeevt := func(e *ffigraph.EventInfo) *events.Message {
			switch e.State {
			case ffigraph.Popped:
				return events.NewTaskCompleted(e.Parent, e.ID, "completed")
			case ffigraph.Pushed:
				return events.NewTaskInitiated(e.Parent, e.ID, "initiated")
			default:
				return events.NewTaskErrored(e.ID, fmt.Sprintf("unknown %d", e.State))
			}
		}

		c := events.NewEventsClient(cc)
		for {
			select {
			case <-gctx.Context.Done():
				return
			case evt := <-ebuf:
				if _, err := c.Dispatch(gctx.Context, events.NewDispatch(makeevt(evt))); err != nil {
					log.Println("unable to dispatch event", err, spew.Sdump(evt))
					continue
				}
			}
		}
	}()

	return interp.Remote(
		gctx.Context,
		aid,
		uid,
		ffigraph.NewListener(ebuf),
		cc,
		t.Dir,
		t.Module,
		interp.OptionRuntimeDir(t.RuntimeDir),
	)
}
