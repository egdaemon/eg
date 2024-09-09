package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	const cname = "eg.c8s.workload"
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Build(eg.Container(cname).BuildFromFile("workspace/Containerfile")),
		eg.Exec(ctx, eg.Container(cname)),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
