package main

import (
	"context"
	"log"
	"path/filepath"
	"slices"

	"eg/compute/archlinux"
	debeg "eg/compute/debuild/eg"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	slices.Backward()
	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Parallel(
			debeg.Prepare,
			eg.Build(eg.Container(archlinux.ContainerName).BuildFromFile(".dist/archlinux/Containerfile")),
		),
		eg.Parallel(
			eg.Module(
				ctx,
				debeg.Runner(),
				eg.Sequential(
					debeg.Build,
					debeg.Upload,
					shell.Op(
						shell.Newf("cp %s/*.deb %s", egenv.EphemeralDirectory("deb.eg"), egenv.CacheDirectory(".dist")),
					),
				),
			),
			eg.Module(ctx, archlinux.Builder(archlinux.ContainerName), archlinux.Build),
		),
		eggithub.Release(
			append(
				errorsx.Zero(filepath.Glob(egenv.CacheDirectory(".dist", "*.deb"))),
				slicesx.Last(errorsx.Zero(filepath.Glob(egenv.CacheDirectory(".dist", "pacman")))...),
			)...,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
