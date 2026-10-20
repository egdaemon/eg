// Package localref provides tests for modules whose operations reference variables declared outside of them.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	const foo = "bar"
	// derived from foo, ensures dependencies of referenced declarations are transferred as well.
	expected := "value-" + foo

	err := eg.Perform(
		ctx,
		eg.Build(eg.DefaultModule()),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				return shell.Run(ctx, shell.Newf("test %s = %s", foo, "bar"))
			},
			func(ctx context.Context, op eg.Op) error {
				return shell.Run(ctx, shell.Newf("test %s = %s", expected, "value-bar"))
			},
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
