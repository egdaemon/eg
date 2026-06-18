package main

import (
	"context"
	"log"

	"eg/compute/debuild/llamacpp"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		llamacpp.Prepare,
		eg.Module(
			ctx,
			llamacpp.Runner(),
			llamacpp.Build,
			llamacpp.Upload,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
