package main

import (
	"context"
	"fmt"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	const prefix = "value"
	unused := "not referenced by the module"
	name := fmt.Sprintf("%s-%s", prefix, "bar")
	_ = name
	expected := name + "!"
	_ = expected

	log.Println(unused)

	err := eg.Perform(
		ctx, eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.25.3.wasm`,
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
