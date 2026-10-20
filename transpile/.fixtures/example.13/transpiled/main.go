// closures nested within the operation capture through the operation.
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

	prefix := "p"
	_ = prefix
	suffix := "s"
	_ = suffix

	err := eg.Perform(
		ctx, eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.21.3.wasm`,
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
