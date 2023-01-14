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
)

type runner struct {
	Dir       string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir string `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
	CacheDir  string `name:"builddir" help:"directory to output modules relative to the module directory" default:".cache"`
}

func (t runner) Run(ctx *cmdopts.Global) (err error) {
	builddir := filepath.Join(t.ModuleDir, t.CacheDir, "build")

	cacheid, roots, err := transpile.Autodetect().Run(ctx.Context, transpile.New(os.DirFS(t.Dir), t.ModuleDir, t.CacheDir))
	if err != nil {
		return err
	}

	log.Println("cacheid", cacheid, "roots", roots)

	for _, root := range roots {
		var (
			path string
		)

		if path, err = filepath.Rel(filepath.Join(t.ModuleDir, t.CacheDir, "transpile"), root); err != nil {
			return err
		}

		path = strings.TrimSuffix(path, filepath.Ext(path)) + ".wasm"
		path = filepath.Join(builddir, path)

		if err = compile.Run(ctx.Context, root, path); err != nil {
			return err
		}
	}

	return interp.Run(ctx.Context, t.Dir, interp.OptionModuleDir(t.ModuleDir), interp.OptionBuildDir(builddir))
}
