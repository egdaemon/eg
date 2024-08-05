package main

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/interp"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigraph"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid"
	"google.golang.org/grpc"
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

	if err = gitx.AutomaticCredentialRefresh(gctx.Context, tlsc.DefaultClient(), t.RuntimeDir, envx.String("", gitx.EnvAuthEGAccessToken)); err != nil {
		return err
	}

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

type monitor struct {
	RunID string `name:"runid"`
}

func (t monitor) Run(ctx *cmdopts.Global) (err error) {
	var (
		cc    *grpc.ClientConn
		grpcl net.Listener
	)

	if grpcl, err = daemons.DefaultAgentListener(); err != nil {
		return err
	}
	defer grpcl.Close()

	if err = daemons.Agent(ctx, grpcl); err != nil {
		return err
	}

	if cc, err = daemons.DefaultRunnerClient(ctx.Context); err != nil {
		return err
	}

	w, err := events.NewAgentClient(cc).Watch(ctx.Context, &events.RunWatchRequest{Run: &events.RunMetadata{Id: uuid.FromStringOrNil(t.RunID).Bytes()}})
	if err != nil {
		return err
	}

	// p := tea.NewProgram(
	// 	ux.NewGraph(),
	// 	tea.WithoutSignalHandler(),
	// 	tea.WithContext(ctx.Context),
	// )

	go func() {
		for {
			select {
			case <-ctx.Context.Done():
				return
			default:
			}

			m, err := w.Recv()
			if err == io.EOF {
				log.Println("EOF received")
				return
			} else if err != nil {
				log.Println("unable to receive message", err)
				continue
			}

			log.Println("evt", spew.Sdump(m))
			// p.Send(m)
		}
	}()

	// go func() {
	// 	<-ctx.Context.Done()
	// 	p.Send(tea.Quit)
	// }()

	// if _, err := p.Run(); err != nil {
	// 	return err
	// }

	return nil
}
