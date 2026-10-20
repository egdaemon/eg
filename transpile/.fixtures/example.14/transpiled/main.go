// modules declared in functions other than main.
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
	return eg.Perform(
		ctx, eg.UnsafeRunner(ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.16.3.wasm`),
	)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	if err := eg.Perform(ctx, Helper); err != nil {
		log.Fatalln(err)
	}
}
