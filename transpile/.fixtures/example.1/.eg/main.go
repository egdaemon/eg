package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	const foo = "bar"

	err := eg.Perform(
		ctx,
		eg.Module(ctx, eg.DefaultModule(), func(ctx context.Context, op eg.Op) error {
			log.Println("DERP", foo)
			return nil
		}),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
