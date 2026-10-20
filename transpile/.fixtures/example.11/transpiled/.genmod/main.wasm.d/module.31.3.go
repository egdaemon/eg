package main

import (
	"context"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
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
	if err := eg.Perform(ctx, eg.Sequential(Named, func(ctx context.Context, op eg.Op) error {
		log.Println("inline", timeout)
		return nil
	}, Build(timeout))); err != nil {
		log.Fatalln(err)
	}
}
