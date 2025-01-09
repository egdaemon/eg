// Package envvars provides tests for nesting behaviors of environment variables for modules.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

func Test(depth int) eg.OpFn {
	return func(ctx context.Context, op eg.Op) error {
		return shell.Run(
			ctx,
			shell.Newf("test %d -eq %d", egbug.Depth(), depth),
		)
	}
}

func Level0(ctx context.Context, op eg.Op) error {
	return eg.Perform(ctx, egbug.Module, Test(0), eg.Module(ctx, eg.DefaultModule(), Level1))
}

func Level1(ctx context.Context, op eg.Op) error {
	return eg.Perform(ctx, egbug.Module, Test(1), eg.Module(ctx, eg.DefaultModule(), Level2))
}

func Level2(ctx context.Context, op eg.Op) error {
	return eg.Perform(ctx, egbug.Module, Test(2))
}

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	if err := eg.Perform(ctx, eg.Build(eg.DefaultModule()), Level0); err != nil {
		log.Fatalln(err)
	}
}
