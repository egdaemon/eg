package main

import (
	"context"
	"log"
	"path/filepath"

	debeg "eg/compute/debuild/eg"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Parallel(
			debeg.Prepare,
			//  eg.Build(eg.Container(archlinux.ContainerName).BuildFromFile(".dist/archlinux/Containerfile")),
		),
		eg.Parallel(
			eg.Module(
				ctx,
				debeg.Runner(),
				eg.Sequential(
					debeg.Build,
					debeg.Upload,
					shell.Op(
						// shell.Newf("tree -L 2 -a %s", egenv.EphemeralDirectory("deb.eg")).Privileged(),
						shell.Newf("cp %s/*.deb %s", egenv.EphemeralDirectory("deb.eg"), egenv.CacheDirectory(".dist")),
					),
				),
			),
			// eg.Module(ctx, archlinux.Builder(archlinux.ContainerName), archlinux.Build),
		),
		eggithub.Release(errorsx.Zero(filepath.Glob(egenv.CacheDirectory(".dist", "*.deb")))...),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
