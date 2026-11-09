// guest used by the ffiegvm tests: asks the host to run a module in a vm, exiting non-zero when it refuses.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegvm"
)

func main() {
	if err := ffiegvm.Run(context.Background(), "https://example.com/root.egiso", "sha256:abc", "module.wasm", "A=1", "B=2"); err != nil {
		log.Fatalln(err)
	}
}
