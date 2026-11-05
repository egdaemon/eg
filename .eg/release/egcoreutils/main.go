package main

import (
	"context"
	"log"

	"eg/compute/debuild/egcoreutils"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		egcoreutils.Prepare,
		eg.Module(
			ctx,
			egcoreutils.Runner(),
			egcoreutils.Build,
			egcoreutils.Upload,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
