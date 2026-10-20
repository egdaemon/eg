// declarations from other files of the package and nested packages.
package main

import (
	"context"
	"log"

	"eg/compute/m1"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	local := Shared + "!"

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				log.Println(local, Shared)
				return nil
			},
			m1.Root,
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
