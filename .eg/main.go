package main

import (
	"context"
	"eg/ci/debian"
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

	deb := eg.Container(debian.ContainerName)
	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Parallel(
			eg.Build(eg.DefaultModule()),
			eg.Build(deb.BuildFromFile(".dist/deb/Containerfile")),
		),
		eg.Module(
			ctx,
			deb,
			// egbug.FileTree,
			eggolang.AutoCompile(
				eggolang.CompileOption.BuildOptions(
					eggolang.Build(
						eggolang.BuildOption.Tags("no_duckdb_arrow"),
					),
				),
			),
			eggolang.AutoTest(
				eggolang.TestOption.BuildOptions(
					eggolang.Build(
						eggolang.BuildOption.Tags("no_duckdb_arrow"),
					),
				),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
