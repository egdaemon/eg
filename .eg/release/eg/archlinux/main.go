package main

import (
	"context"
	"eg/compute/archlinux"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
	"github.com/egdaemon/eg/runtime/x/wasi/egerrors"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
)

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
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
				archlinux.Build,
				eggithub.Draft(
					egenv.DirectoryCache(".dist", "pacman", "*.pkg.tar.*"),
					archlinux.PKGBUILD(),
				),
				egerrors.Chain(
					eg.Fn(archlinux.Publish),
					egbug.Log("our archlinux ssh keys are currently rejected, need to talk to upstream"),
				),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
