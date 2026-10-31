// Package envvars provides tests for the default environment of modules.
package main

import (
	"context"
	"log"
	"os"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()
	egfs.Inspect(ctx, os.DirFS(egunsafe.RuntimeDirectory()))
	if err := eg.Perform(
		ctx,
		egbug.EnsureEnvAuto,
	); err != nil {
		log.Fatalln(err)
	}
}
