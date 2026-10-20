package main

import (
	"context"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func First(ctx context.Context, op eg.Op) error {
	return nil
}
func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()
	const msg = "m"
	if err := eg.Perform(ctx, eg.Sequential(eg.Sequential(First, func(ctx context.Context, op eg.Op) error {
		log.Println(msg)
		return nil
	}))); err != nil {
		log.Fatalln(err)
	}
}
