package actl

import (
	"embed"
	"path/filepath"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/fsx"
)

//go:embed .bootstrap.module
var embeddedbootstrapmodule embed.FS

type BootstrapModule struct {
	Dir      string `name:"directory" help:"root directory to use" default:"${vars_cwd}"`
	Relative string `name:"relative" help:"relative path from the directory to create the module within" default:".eg"`
}

func (t BootstrapModule) Run(gctx *cmdopts.Global) (err error) {
	egdir := filepath.Join(t.Dir, t.Relative)
	if err = fsx.CloneTree(gctx.Context, egdir, ".bootstrap.module", embeddedbootstrapmodule); err != nil {
		return err
	}

	if err = compile.InitGolang(gctx.Context, egdir, cmdopts.ModPath()); err != nil {
		return err
	}

	if err = compile.InitGolangTidy(gctx.Context, egdir); err != nil {
		return err
	}

	return nil
}
