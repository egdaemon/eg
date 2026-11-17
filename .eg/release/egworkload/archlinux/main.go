package main

import (
	"context"
	"log"

	"eg/compute/archlinux"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile | log.LUTC)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Build(eg.Container(archlinux.ContainerName).BuildFromFile(".dist/archlinux/Containerfile")),
		eg.Module(
			ctx,
			archlinux.Builder(archlinux.ContainerName),
			eg.Sequential(
				archlinux.BuildWorkload,
				eggithub.Draft(egenv.DirectoryCache(".dist", "pacman", "egworkload-*.pkg.tar.*")),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
