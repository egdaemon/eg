// ctx and done are declared by the generated module and are not copied.
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

	deadline, ok := ctx.Deadline()

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				log.Println(ctx.Err(), deadline, ok)
				return nil
			},
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
