// local type declarations: structs and aliases.
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

	type config struct {
		Name  string
		Ports []int
	}
	type alias = map[string]config
	cfg := config{Name: "example", Ports: []int{80, 443}}
	_ = cfg
	lookup := alias{"example": cfg}
	_ = lookup

	err := eg.Perform(
		ctx, eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.26.3.wasm`,
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
