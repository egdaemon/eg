package main

import (
	"context"
	"log"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
)

func main() {
	const cname = "eg.c8s.workload"
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.When(func(ctx context.Context) bool { return egenv.Boolean(false, _eg.EnvComputeContainerImpure) }, eggit.AutoClone),
		eg.Build(eg.Container(cname).BuildFromFile(egenv.RuntimeDirectory("workspace", "Containerfile"))),
		eg.Exec(ctx, eg.Container(cname)),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
