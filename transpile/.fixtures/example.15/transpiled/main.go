// aliased imports of eg and egenv, with captured declarations using other packages.
package main

import (
	"context"
	"log"
	"strings"
	"time"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	egalias "github.com/egdaemon/eg/runtime/wasi/eg"
	envalias "github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), envalias.TTL())
	defer done()

	upper := strings.ToUpper("value")
	_ = upper
	wait := 2 * time.Second
	_ = wait

	err := egalias.Perform(
		ctx, egalias.UnsafeRunner(ctx, egalias.DefaultModule().ToModuleRunner(), `main.wasm.d/module.23.3.wasm`),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
