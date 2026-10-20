// variable declaration forms: typed, multiple names and grouped.
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

	var count, total int = 1, 2
	var (
		name  string = "n"
		ratio        = 0.5
	)
	var empty []string

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				log.Println(count, total, name, ratio, empty)
				return nil
			},
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
