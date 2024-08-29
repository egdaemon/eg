package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
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
)

type module struct {
	Dir        string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir  string `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
	RuntimeDir string `name:"runtimedir" help:"runtime directory" hidden:"true" default:"/opt/egruntime/"`
	Module     string `arg:"" help:"name of the module to run"`
}

func (t module) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		ws   workspaces.Context
		uid  = envx.String(uuid.Nil.String(), "EG_RUN_ID")
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
			control net.Listener
			db      *sql.DB
		)

		log.Println("---------------------------- ROOT MODULE INITIATED ----------------------------")
		defer log.Println("---------------------------- ROOT MODULE COMPLETED ----------------------------")

		cspath := filepath.Join(ws.Root, ws.RuntimeDir, "control.socket")
		if control, err = net.Listen("unix", cspath); err != nil {
			return errorsx.Wrap(err, "unable to create control.socket")
		}
		srv := grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()), // this is a local socket
		)

		if db, err = sql.Open("duckdb", filepath.Join(ws.Root, ws.RuntimeDir, "analytics.db")); err != nil {
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

		containerOpts := []string{}

		c8s.NewServiceProxy(
			log.Default(),
			ws,
			c8s.ServiceProxyOptionCommandEnviron(
				errorsx.Zero(
					envx.Build().FromEnv("PATH", "TERM", "COLORTERM", "LANG", "CI", "EG_CI", "EG_RUN_ID").Environ(),
				)...,
			),
			c8s.ServiceProxyOptionContainerOptions(
				containerOpts...,
			),
		).Bind(srv)

		go func() {
			if err = srv.Serve(control); err != nil {
				log.Println(errorsx.Wrap(err, "unable to create control socket"))
				return
			}
		}()
		defer func() {
			<-gctx.Context.Done()
			srv.GracefulStop()
		}()
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
		uid,
		ffigraph.NewListener(ebuf),
		cc,
		t.Dir,
		t.Module,
		interp.OptionModuleDir(t.ModuleDir),
		interp.OptionRuntimeDir(t.RuntimeDir),
	)
}
