// operations already wrapped in eg.Sequential are wrapped again.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func First(ctx context.Context, op eg.Op) error {
	return nil
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	const msg = "m"

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			eg.Sequential(First, func(ctx context.Context, op eg.Op) error {
				log.Println(msg)
				return nil
			}),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
