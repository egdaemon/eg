// Package envvars provides tests for the default environment of modules.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	if err := eg.Perform(
		ctx,
		egbug.EnsureEnvAuto,
		// egbug.Debug(egbug.EnsureCgroups, egbug.Cgroups),
	); err != nil {
		log.Fatalln(err)
	}
}
