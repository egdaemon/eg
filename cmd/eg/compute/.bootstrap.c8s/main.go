package main

import (
	"context"
	"log"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

func main() {
	const cname = "eg.c8s.workload"
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		egbug.Env,
		eg.When(egenv.Boolean(false, _eg.EnvComputeContainerImpure), eggit.AutoClone),
		eg.Build(eg.Container(cname).BuildFromFile("workspace/Containerfile")),
		eg.Exec(ctx, eg.Container(cname)),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
