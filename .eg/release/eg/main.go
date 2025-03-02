package main

import (
	"context"
	"log"

	"eg/compute/archlinux"
	debeg "eg/compute/debuild/eg"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		egbug.FileTree,
		eg.Parallel(
			eg.Build(eg.Container(archlinux.ContainerName).BuildFromFile(".dist/archlinux/Containerfile")),
		),
		debeg.Prepare,
		eg.Parallel(
			eg.Module(ctx, debeg.Runner(), debeg.Build, debeg.Upload),
			eg.Module(ctx, archlinux.Builder(archlinux.ContainerName), archlinux.Build),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
