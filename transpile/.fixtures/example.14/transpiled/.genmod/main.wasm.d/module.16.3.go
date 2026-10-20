package main

import (
	"context"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func Helper(ctx context.Context, op eg.Op) error {
	message := "from helper"
	_ = message
	return eg.Perform(ctx, eg.UnsafeRunner(ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.16.3.wasm`))
}
func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()
	message := "from helper"
	if err := eg.Perform(ctx, eg.Sequential(func(ctx context.Context, op eg.Op) error {
		log.Println(message)
		return nil
	})); err != nil {
		log.Fatalln(err)
	}
}
