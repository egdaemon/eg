package main

import (
	"context"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()
	deadline, ok := ctx.Deadline()
	if err := eg.Perform(ctx, eg.Sequential(func(ctx context.Context, op eg.Op) error {
		log.Println(ctx.Err(), deadline, ok)
		return nil
	})); err != nil {
		log.Fatalln(err)
	}
}
