package main

import (
	"context"
	"log"

	debian "eg/compute/debuild/eg"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	deb := debian.Runner()
	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Parallel(
			eg.Build(eg.DefaultModule()),
			eg.Build(deb.BuildFromFile(".eg/debuild/eg/.debskel/Containerfile")),
		),
		eg.Module(
			ctx,
			deb,
			eggolang.AutoCompile(),
			eggolang.AutoTest(),
			eggolang.RecordCoverage,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
