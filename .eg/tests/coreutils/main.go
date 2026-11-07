// Package main exercises the egcoreutils reactor through fficoreutils, the reactor must be installed in the wasi
// module directory as egcoreutils.wasi (see cmd/eg/README.wasi.md).
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/x/wasi/egbash"
	"github.com/egdaemon/eg/runtime/x/wasi/egerrors"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile | log.LUTC | log.Ldate)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := egbash.Run(
		ctx,
		egbash.New("echo 'hello world' | tee derp.log"),
		egbash.New("echo 'reading derp.log:'; cat derp.log"),
	)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("fin", egerrors.Zero(egfs.String("derp.log")))
}
