// each module only receives the declarations it references.
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

	a := "a"
	_ = a
	b := "b"
	_ = b

	err := eg.Perform(
		ctx, eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.21.3.wasm`,
		), eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.29.3.wasm`,
		), eg.Parallel(eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.38.4.wasm`,
		),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
