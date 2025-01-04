package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

func TestNetwork(ctx context.Context, op eg.Op) error {
	return shell.Run(
		ctx,
		// shell.New("eg compute local tests/network"),
	)
}

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
		eggolang.AutoCompile(
			eggolang.CompileOptionTags("no_duckdb_arrow"),
		),
		eg.Parallel(
			eg.Module(
				ctx,
				eg.DefaultModule(),
				eggolang.AutoTest(
					eggolang.TestOptionTags("no_duckdb_arrow"),
				),
			),
			// TestNetwork, // TODO
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
