// multi value definitions are transferred as a whole statement.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func split(s string) (string, string) {
	return s, ""
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	host, port := split("localhost:8080")

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				log.Println(host)
				return nil
			},
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
