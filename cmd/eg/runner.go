package main

import (
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/awalterschulze/gographviz"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dominikbraun/graph"
	"github.com/gofrs/uuid"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/cmd/ux"
	"github.com/james-lawrence/eg/compile"
	"github.com/james-lawrence/eg/interp"
	"github.com/james-lawrence/eg/transpile"
	"github.com/james-lawrence/eg/workspaces"
	"github.com/pkg/errors"
)

type runner struct {
	Dir       string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir string `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
}

func (t runner) Run(ctx *cmdopts.Global) (err error) {
	var (
		ws workspaces.Context
	)

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

	gg := gographviz.NewGraph()
	gg.Directed = true

	for _, m := range modules {
		if m.Generated {
			continue
		}

		if err = interp.Analyse(ctx.Context, gg, uuid.Must(uuid.NewV4()).String(), t.Dir, m.Path, interp.OptionModuleDir(t.ModuleDir)); err != nil {
			return errors.Wrapf(err, "failed to analyse module %s", m.Path)
		}
	}

	for _, m := range modules {
		if m.Generated {
			continue
		}

		if err = interp.Run(ctx.Context, uuid.Must(uuid.NewV4()).String(), t.Dir, m.Path, interp.OptionModuleDir(t.ModuleDir)); err != nil {
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
	return interp.Run(ctx.Context, uuid.Must(uuid.NewV4()).String(), t.Dir, t.Module, interp.OptionModuleDir(t.ModuleDir))
}

type monitor struct {
	Dir       string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir string `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
}

func (t monitor) Run(ctx *cmdopts.Global) (err error) {
	const (
		path = "derp.dot"
	)

	var (
		fh   *os.File
		rawg []byte
		gg   *gographviz.Graph
	)

	if fh, err = os.Open(path); err != nil {
		log.Fatalln(err)
	}
	defer fh.Close()

	if rawg, err = io.ReadAll(fh); err != nil {
		log.Fatalln(err)
	}

	if gg, err = gographviz.Read(rawg); err != nil {
		log.Fatalln(err)
	}

	g, err := ux.TranslateGraphiz(gg)
	if err != nil {
		return err
	}

	p := tea.NewProgram(
		ux.NewGraph(g),
		tea.WithoutSignalHandler(),
		tea.WithContext(ctx.Context),
	)
	go func() {
		for {
			time.Sleep(time.Second)

			nodes, _ := graph.TopologicalSort(g)
			if len(nodes) == 0 {
				continue
			}

			choice := rand.Intn(len(nodes))
			n, _ := g.Vertex(nodes[choice])
			p.Send(ux.EventTask{ID: n.ID, State: (n.State + 1) % (ux.StateError + 1)})
		}
	}()

	go func() {
		<-ctx.Context.Done()
		p.Send(tea.Quit)
	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	// err = draw.DOT(g, langx.Must(os.OpenFile("derp2.dot", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)))
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	return nil
}
