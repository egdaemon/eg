// captured values providing operations through method values.
package main

import (
	"context"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

type Runner struct {
	name string
}

func (r Runner) Run(ctx context.Context, op eg.Op) error {
	log.Println(r.name)
	return nil
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	r := Runner{name: "r"}
	_ = r

	err := eg.Perform(
		ctx, eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.29.3.wasm`,
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
