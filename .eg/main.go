package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

func TestNetwork(ctx context.Context, op eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("eg compute local tests/network"),
	)
}

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()
	err := eg.Perform(
		ctx,
		egbug.Users,
	)
	// c1 := eg.Container("eg.ubuntu.24.10")
	// err := eg.Perform(
	// 	ctx,
	// 	eggit.AutoClone,
	// 	eg.Build(
	// 		c1.BuildFromFile(".dist/deb/Containerfile"),
	// 	),
	// 	eg.Parallel(
	// 		eg.Module(
	// 			ctx,
	// 			c1,
	// 			eggolang.AutoTest(
	// 				eggolang.TestOptionTags("no_duckdb_arrow"),
	// 			),
	// 		),
	// 		// TestNetwork, // TODO
	// 	),
	// )

	if err != nil {
		log.Fatalln(err)
	}
}
