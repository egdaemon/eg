package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gofrs/uuid"
	"github.com/james-lawrence/eg/cmd/cmdopts"
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

	for _, m := range modules {
		if m.Generated {
			continue
		}

		if err = interp.Analyse(ctx.Context, uuid.Must(uuid.NewV4()).String(), t.Dir, m.Path, interp.OptionModuleDir(t.ModuleDir)); err != nil {
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
