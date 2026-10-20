package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	const foo = "bar"
	// only referenced by the module, the host program must still compile.
	expected := "value-" + foo

	err := eg.Perform(
		ctx,
		eg.Module(ctx, eg.DefaultModule(), func(ctx context.Context, op eg.Op) error {
			log.Println("DERP", foo, expected)
			return nil
		}),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
