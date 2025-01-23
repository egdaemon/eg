package main

import (
	"context"
	"log"

	debian "eg/compute/debuild/eg"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		egbug.FileTree,
		shell.Op(
			shell.Newf("mkdir -p %s", egenv.CacheDirectory(".dist")),
		),
		eg.Parallel(
			eg.Build(eg.Container(debian.ContainerName).BuildFromFile(".dist/deb/Containerfile")),
		// eg.Build(eg.Container(archlinux.ContainerName).BuildFromFile(".dist/archlinux/Containerfile")),
		),
		eg.Parallel(
			eg.Module(ctx, debian.Builder(debian.ContainerName, "jammy"), debian.Build),
			eg.Module(ctx, debian.Builder(debian.ContainerName, "noble"), debian.Build),
			eg.Module(ctx, debian.Builder(debian.ContainerName, "oracular"), debian.Build),
		// eg.Module(ctx, archlinux.Builder(archlinux.ContainerName), archlinux.Build),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
