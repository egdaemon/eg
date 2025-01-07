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

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Build(
			eg.DefaultModule(),
		),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			eggolang.AutoCompile(
				eggolang.CompileOption.Tags("no_duckdb_arrow"),
				// eggolang.CompileOption.Debug(true),
			),
			eggolang.AutoTest(
				eggolang.TestOption.Tags("no_duckdb_arrow"),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
