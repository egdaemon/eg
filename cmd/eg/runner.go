package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/compile"
	"github.com/james-lawrence/eg/interp"
)

type runner struct {
	Dir        string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir  string `name:"moduledir" help:"directory to load eg modules from" default:".eg"`
	BuildCache string `name:"builddir" help:"directory to output modules relative to the module directory" default:".cache"`
}

func (t runner) Run(ctx *cmdopts.Global) (err error) {
	err = fs.WalkDir(os.DirFS(t.Dir), t.ModuleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// TODO: identify code to compile
		log.Println("visiting", path)
		return nil
	})

	if err != nil {
		return err
	}

	moduledir := filepath.Join(t.ModuleDir, t.BuildCache, "build")
	if err = compile.Run(ctx.Context, filepath.Join(t.Dir, t.ModuleDir, "example1/main.go"), filepath.Join(t.Dir, moduledir, "eg1.example.wasm")); err != nil {
		return err
	}

	return interp.Run(ctx.Context, t.Dir, interp.OptionModuleDir(moduledir))
}
