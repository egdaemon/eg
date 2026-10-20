package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func First(ctx context.Context, op eg.Op) error {
	log.Println("first")
	return nil
}

func Second(ctx context.Context, op eg.Op) error {
	log.Println("second")
	return nil
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Module(ctx, eg.DefaultModule(), First, Second),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
