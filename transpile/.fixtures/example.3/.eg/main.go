package main

import (
	"context"
	"fmt"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	const prefix = "value"
	unused := "not referenced by the module"
	name := fmt.Sprintf("%s-%s", prefix, "bar")
	expected := name + "!"

	log.Println(unused)

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			func(ctx context.Context, op eg.Op) error {
				log.Println("first", expected)
				return nil
			},
			func(ctx context.Context, op eg.Op) error {
				log.Println("second", prefix)
				return nil
			},
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
