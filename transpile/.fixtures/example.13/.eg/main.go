// closures nested within the operation capture through the operation.
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

	prefix := "p"
	suffix := "s"

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				wrap := func(s string) string {
					return func() string { return prefix + s + suffix }()
				}
				log.Println(wrap("x"))
				return nil
			},
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
