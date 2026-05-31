package main

import (
	"context"
	"log"

	"eg/compute/debuild/egworkload"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
)

func init() {
	log.SetFlags(log.Flags() | log.Lshortfile | log.LUTC)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		egworkload.Prepare,
		eg.Module(
			ctx,
			egworkload.Runner(),
			egworkload.Build,
			egworkload.Upload,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
