package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davecgh/go-spew/spew"
	"github.com/gofrs/uuid"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/cmd/eg/daemons"
	"github.com/james-lawrence/eg/cmd/ux"
	"github.com/james-lawrence/eg/compile"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/interp"
	"github.com/james-lawrence/eg/interp/events"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffigraph"
	"github.com/james-lawrence/eg/transpile"
	"github.com/james-lawrence/eg/workspaces"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type runner struct {
	Dir       string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir string `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
}

func (t runner) Run(ctx *cmdopts.Global) (err error) {
	var (
		ws   workspaces.Context
		uid  = uuid.Must(uuid.NewV7())
		ebuf = make(chan *ffigraph.EventInfo)
		cc   grpc.ClientConnInterface
	)

	if cc, err = daemons.AutoRunnerClient(ctx, uid.String()); err != nil {
		return err
	}

	// enable event logging
	// w, err := events.NewAgentClient(cc).Watch(ctx.Context, &events.RunWatchRequest{Run: &events.RunMetadata{Id: uid.Bytes()}})
	// if err != nil {
	// 	return err
	// }

	// go func() {
	// 	for {
	// 		select {
	// 		case <-ctx.Context.Done():
	// 			return
	// 		default:
	// 		}

	// 		m, err := w.Recv()
	// 		if err == io.EOF {
	// 			log.Println("EOF received")
	// 			return
	// 		} else if err != nil {
	// 			log.Println("unable to receive message", err)
	// 			continue
	// 		}

	// 		log.Println("DERP", spew.Sdump(m))
	// 	}
	// }()

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
			case <-ctx.Context.Done():
				return
			case evt := <-ebuf:
				if _, err := c.Dispatch(ctx.Context, events.NewDispatch(makeevt(evt))); err != nil {
					log.Println("unable to dispatch event", err, spew.Sdump(evt))
					continue
				}
			}
		}
	}()

	if ws, err = workspaces.New(ctx.Context, t.Dir, t.ModuleDir); err != nil {
		return err
	}

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(ctx.Context)
	if err != nil {
		return err
	}

	log.Println("cacheid", ws.CachedID)

	modules := make([]transpile.Compiled, 0, len(roots))

	for _, root := range roots {
		var (
			path string
		)

		if path, err = filepath.Rel(ws.TransDir, root.Path); err != nil {
			return err
		}

		path = workspaces.TrimRoot(path, filepath.Base(ws.GenModDir))
		path = workspaces.ReplaceExt(path, ".wasm")
		path = filepath.Join(ws.BuildDir, path)
		modules = append(modules, transpile.Compiled{Path: path, Generated: root.Generated})

		if _, err = os.Stat(path); err == nil {
			// nothing to do.
			continue
		}

		if err = compile.Run(ctx.Context, root.Path, path); err != nil {
			return err
		}
	}

	log.Println("roots", roots)
	// TODO: run the modules inside a container for safety
	// {
	// 	cmd := exec.CommandContext(ctx.Context, "podman", "build", "--timestamp", "0", "-t", "ubuntu:jammy", t.Dir)
	// 	log.Println("RUNNING", cmd.String())
	// 	if err = cmd.Run(); err != nil {
	// 		log.Println("DERP 1", err)
	// 		return err
	// 	}
	// }

	for _, m := range modules {
		if m.Generated {
			continue
		}

		if err = interp.Analyse(ctx.Context, ffigraph.NewListener(ebuf), uid.String(), t.Dir, m.Path, interp.OptionModuleDir(t.ModuleDir)); err != nil {
			return errors.Wrapf(err, "failed to analyse module %s", m.Path)
		}
	}

	for _, m := range modules {
		if m.Generated {
			continue
		}

		if err = interp.Run(ctx.Context, uid.String(), ffigraph.NewListener(ebuf), t.Dir, m.Path, interp.OptionModuleDir(t.ModuleDir)); err != nil {
			return errors.Wrapf(err, "failed to run module %s", m.Path)
		}
	}

	return nil
}

type module struct {
	Dir       string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir string `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
	Module    string `arg:"" help:"name of the module to run"`
}

func (t module) Run(ctx *cmdopts.Global) (err error) {
	var (
		uid  = envx.String(uuid.Must(uuid.NewV7()).String(), "EG_RUN_ID")
		ebuf = make(chan *ffigraph.EventInfo)
		cc   grpc.ClientConnInterface
	)

	if cc, err = daemons.AutoRunnerClient(ctx, uid); err != nil {
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
			case <-ctx.Context.Done():
				return
			case evt := <-ebuf:
				if _, err := c.Dispatch(ctx.Context, events.NewDispatch(makeevt(evt))); err != nil {
					log.Println("unable to dispatch event", spew.Sdump(evt))
					continue
				}
			}
		}
	}()

	return interp.Run(ctx.Context, uid, ffigraph.NewListener(ebuf), t.Dir, t.Module, interp.OptionModuleDir(t.ModuleDir))
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

	p := tea.NewProgram(
		ux.NewGraph(),
		tea.WithoutSignalHandler(),
		tea.WithContext(ctx.Context),
	)

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

			p.Send(m)
		}
	}()

	go func() {
		<-ctx.Context.Done()
		p.Send(tea.Quit)
	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
