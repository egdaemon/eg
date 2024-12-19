package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	c1 := egbug.NewCounter()
	mods := eggit.NewModified()

	err := eg.Perform(
		ctx,
		eg.When(mods.Changed(), c1.Op),
		c1.Assert(1),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
