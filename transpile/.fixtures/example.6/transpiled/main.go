// multi value definitions are transferred as a whole statement.
package main

import (
	"context"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func split(s string) (string, string) {
	return s, ""
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	host, port := split("localhost:8080")
	_ = host
	_ = port

	err := eg.Perform(
		ctx, eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.24.3.wasm`,
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
