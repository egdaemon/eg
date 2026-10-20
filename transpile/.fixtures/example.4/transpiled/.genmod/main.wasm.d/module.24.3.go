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
	const (
		first = iota
		second
		third
	)
	if err := eg.Perform(ctx, eg.Sequential(func(ctx context.Context, op eg.Op) error {
		log.Println(second)
		return nil
	})); err != nil {
		log.Fatalln(err)
	}
}
