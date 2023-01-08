package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
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
	content := md5.New()
	err = fs.WalkDir(os.DirFS(t.Dir), t.ModuleDir, func(path string, d fs.DirEntry, err error) error {
		var (
			c *os.File
		)

		// TODO: identify code to compile and checksum
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if c, err = os.Open(path); err != nil {
			return err
		}

		if _, err = io.Copy(content, c); err != nil {
			return err
		}

		log.Println("visiting", path)
		return nil
	})

	if err != nil {
		return err
	}

	log.Println("checksum", hex.EncodeToString(content.Sum(nil)))

	builddir := filepath.Join(t.ModuleDir, t.BuildCache, "build")
	if err = compile.Run(ctx.Context, filepath.Join(t.Dir, t.ModuleDir, "example1/main.go"), filepath.Join(t.Dir, builddir, "eg1.example.wasm")); err != nil {
		return err
	}

	return interp.Run(ctx.Context, t.Dir, interp.OptionModuleDir(t.ModuleDir), interp.OptionBuildDir(builddir))
}
