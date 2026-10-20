// aliased imports of eg and egenv, with captured declarations using other packages.
package main

import (
	"context"
	"log"
	"strings"
	"time"

	egalias "github.com/egdaemon/eg/runtime/wasi/eg"
	envalias "github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), envalias.TTL())
	defer done()

	upper := strings.ToUpper("value")
	wait := 2 * time.Second

	err := egalias.Perform(
		ctx,
		egalias.Module(ctx, egalias.DefaultModule(), func(ctx context.Context, op egalias.Op) error {
			log.Println(upper, wait)
			return nil
		}),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
