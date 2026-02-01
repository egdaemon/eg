package main

import (
	"context"
	"eg/compute/m1"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func Debug(ctx context.Context, _ eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("echo root"),
	)
}

func Root(ctx context.Context, _ eg.Op) error {
	c1 := eg.Container("egmeta.ubuntu.24.10")
	return eg.Perform(
		ctx,
		eg.Module(ctx, c1, Debug),
		m1.Debug,
	)
}

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	c1 := eg.Container("egmeta.ubuntu")

	err := eg.Perform(
		ctx,
		eg.Build(
			c1.PullFrom("ubuntu:plucky"),
		),
		eg.Module(ctx, c1, Root),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
