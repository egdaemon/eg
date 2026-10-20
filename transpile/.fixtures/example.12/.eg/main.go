// each module only receives the declarations it references.
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

	a := "a"
	b := "b"

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				log.Println(a)
				return nil
			},
		),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				log.Println(b)
				return nil
			},
		),
		eg.Parallel(
			eg.Module(
				ctx,
				eg.DefaultModule(),
				func(ctx context.Context, op eg.Op) error {
					log.Println(a, b)
					return nil
				},
			),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
