package main

import (
	"context"
	"log"

	"eg/compute/daemon"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	c1 := eg.Container("eg.ubuntu")

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Build(
			c1.BuildFromFile(".eg/Containerfile"),
		),
		eg.Module(
			ctx,
			c1,
			daemon.Gogen,
		),
		shell.Op(
			shell.New("git diff > ${PATCH}").Environ("PATCH", egenv.CacheDirectory("codegen.patch")),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
