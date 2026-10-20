// modules declared in functions other than main.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func Helper(ctx context.Context, op eg.Op) error {
	message := "from helper"
	return eg.Perform(
		ctx,
		eg.Module(ctx, eg.DefaultModule(), func(ctx context.Context, op eg.Op) error {
			log.Println(message)
			return nil
		}),
	)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	if err := eg.Perform(ctx, Helper); err != nil {
		log.Fatalln(err)
	}
}
