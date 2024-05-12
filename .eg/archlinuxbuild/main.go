package main

import (
	"context"
	"eg/ci/archlinux"
	"log"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/shellx"
)

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	if s, err := shellx.String(ctx, "/usr/bin/echo", "hello world"); err == nil {
		log.Println("shell output", s)
	}

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Parallel(
			eg.Build(eg.Container(archlinux.ContainerName).BuildFromFile(".dist/archlinux/Containerfile")),
		),
		eg.Parallel(
			eg.Module(ctx, archlinux.Builder(archlinux.ContainerName), archlinux.Build),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
