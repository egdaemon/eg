package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/compile"
	"github.com/james-lawrence/eg/interp"
	"github.com/james-lawrence/eg/transpile"
	"github.com/james-lawrence/eg/workspaces"
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

	roots, err := transpile.Autodetect().Run(ctx.Context, transpile.New(ws))
	if err != nil {
		return err
	}

	log.Println("cacheid", ws.CachedID, "roots", roots)

	for _, root := range roots {
		var (
			path string
		)

		if path, err = filepath.Rel(ws.TransDir, root); err != nil {
			return err
		}

		path = strings.TrimSuffix(path, filepath.Ext(path)) + ".wasm"
		path = filepath.Join(ws.BuildDir, path)

		if _, err = os.Stat(path); err == nil {
			// nothing to do.
			continue
		}

		if err = compile.Run(ctx.Context, root, path); err != nil {
			return err
		}
	}

	return interp.Run(ctx.Context, t.Dir, interp.OptionModuleDir(t.ModuleDir), interp.OptionBuildDir(ws.BuildDir))
}
