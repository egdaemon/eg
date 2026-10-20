package main

import (
	"context"
	"fmt"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()
	const prefix = "value"
	name := fmt.Sprintf("%s-%s", prefix, "bar")
	expected := name + "!"
	if err := eg.Perform(ctx, eg.Sequential(func(ctx context.Context, op eg.Op) error {
		log.Println("first", expected)
		return nil
	}, func(ctx context.Context, op eg.Op) error {
		log.Println("second", prefix)
		return nil
	})); err != nil {
		log.Fatalln(err)
	}
}
