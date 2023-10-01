package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davecgh/go-spew/spew"
	"github.com/gofrs/uuid"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/cmd/eg/daemons"
	"github.com/james-lawrence/eg/cmd/ux"
	"github.com/james-lawrence/eg/compile"
	"github.com/james-lawrence/eg/interp"
	"github.com/james-lawrence/eg/interp/events"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffigraph"
	"github.com/james-lawrence/eg/runners"
	"github.com/james-lawrence/eg/runtime/wasi/langx"
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
		ws     workspaces.Context
		uid    = uuid.Must(uuid.NewV7())
		ragent *runners.Agent
		ebuf   = make(chan *ffigraph.EventInfo)
	)

	m := runners.NewManager(
		ctx.Context,
		langx.Must(filepath.Abs(runners.DefaultManagerDirectory())),
	)

	if ragent, err = m.NewRun(ctx.Context, uid.String()); err != nil {
		return err
	}

	cc, err := ragent.Dial(ctx.Context)
	if err != nil {
		return err
	}
	go func() {
		makeevt := func(e *ffigraph.EventInfo) *events.Message {
			switch e.State {
			case ffigraph.Popped:
				return events.NewTaskCompleted(e.ID, "completed")
			case ffigraph.Pushed:
				return events.NewTaskInitiated(e.ID, "initiated")
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
				if _, err := c.Dispatch(context.Background(), &events.DispatchRequest{Messages: []*events.Message{
					makeevt(evt),
				}}); err != nil {
					log.Println("unable to dispatch event", spew.Sdump(evt))
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
	return interp.Run(ctx.Context, uuid.Must(uuid.NewV4()).String(), ffigraph.New(), t.Dir, t.Module, interp.OptionModuleDir(t.ModuleDir))
}

type monitor struct {
	Dir       string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir string `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
}

func (t monitor) Run(ctx *cmdopts.Global) (err error) {
	var (
		cc *grpc.ClientConn
	)

	if cc, err = daemons.DefaultAgentClient(ctx.Context); err != nil {
		return err
	}

	p := tea.NewProgram(
		ux.NewGraph(cc),
		tea.WithoutSignalHandler(),
		tea.WithContext(ctx.Context),
	)

	go func() {
		<-ctx.Context.Done()
		p.Send(tea.Quit)
	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
