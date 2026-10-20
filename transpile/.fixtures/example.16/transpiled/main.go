// package level declarations stay in the module file and are never duplicated into main.
package main

import (
	"context"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

const constant = "c"

var global = "g"

type Pair[T any] struct {
	A, B T
}

func Max[T int | float64](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func init() {
	log.Println("init")
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	local := global + "!"
	_ = local

	err := eg.Perform(
		ctx, eg.UnsafeRunner(
			ctx, eg.DefaultModule().ToModuleRunner(), `main.wasm.d/module.39.3.wasm`,
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
