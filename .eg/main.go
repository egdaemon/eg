package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	c1 := eg.Container("eg.ubuntu.24.10")
	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Build(
			c1.BuildFromFile(".dist/deb/Containerfile"),
		),
		eg.Parallel(
			eg.Module(
				ctx,
				c1,
				eggolang.AutoTest(
					eggolang.TestOptionTags("no_duckdb_arrow"),
				),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
