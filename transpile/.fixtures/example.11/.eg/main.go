// operations can be named functions, closures and calls that build operations.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func Named(ctx context.Context, op eg.Op) error {
	return nil
}

func Build(n int) eg.OpFn {
	return func(ctx context.Context, op eg.Op) error {
		log.Println(n)
		return nil
	}
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	const timeout = 5

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			Named,
			func(ctx context.Context, op eg.Op) error {
				log.Println("inline", timeout)
				return nil
			},
			Build(timeout),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
