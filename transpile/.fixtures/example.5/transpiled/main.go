// variable declaration forms: typed, multiple names and grouped.
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
	_ = count
	_ = total
	var (
		name  string = "n"
		ratio        = 0.5
	)
	_ = name
	_ = ratio
	var empty []string
	_ = empty

	err := eg.Perform(
		ctx, eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.25.3.wasm`,
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
