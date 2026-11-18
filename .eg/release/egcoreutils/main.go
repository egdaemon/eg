package main

import (
	"context"
	"log"

	"eg/compute/debuild/egcoreutils"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		egcoreutils.Prepare,
		eg.Module(
			ctx,
			egcoreutils.Runner(),
			egcoreutils.Build,
			egcoreutils.Upload,
			egcoreutils.Copy,
		),
		eggithub.Draft(
			egenv.DirectoryWorkload(".dist", "*.deb"),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
