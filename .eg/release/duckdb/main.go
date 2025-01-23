package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/.eg/debuild/duckdb"
	debian "github.com/egdaemon/eg/.eg/debuild/eg"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Build(eg.Container(debian.ContainerName).BuildFromFile(".dist/deb/Containerfile")),
		eg.Parallel(
			duckdb.Build(),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
