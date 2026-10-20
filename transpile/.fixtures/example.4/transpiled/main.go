// grouped constant declarations: the entire group is transferred.
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

	err := eg.Perform(
		ctx, eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.24.3.wasm`,
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
