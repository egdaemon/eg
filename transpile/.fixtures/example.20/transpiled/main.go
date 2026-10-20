// eg.Exec invocations are rewritten without generating modules.
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

	runner := eg.DefaultModule()
	err := eg.Perform(ctx, eg.UnsafeExec(ctx, runner, `main.wasm.d/module.17.25.wasm`))
	if err != nil {
		log.Fatalln(err)
	}
}
