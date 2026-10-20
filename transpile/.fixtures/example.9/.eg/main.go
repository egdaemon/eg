// shadowing: names declared within the operation are not captured from the enclosing function.
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

	foo := "outer"
	bar := "outer"

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				foo := "inner"
				for bar := 0; bar < 3; bar++ {
					log.Println(foo, bar)
				}
				return nil
			},
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
