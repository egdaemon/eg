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
	var count, total int = 1, 2
	var (
		name  string = "n"
		ratio        = 0.5
	)
	var empty []string
	if err := eg.Perform(ctx, eg.Sequential(func(ctx context.Context, op eg.Op) error {
		log.Println(count, total, name, ratio, empty)
		return nil
	})); err != nil {
		log.Fatalln(err)
	}
}
