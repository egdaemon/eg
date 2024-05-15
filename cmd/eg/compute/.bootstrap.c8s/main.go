package main

import (
	"context"
	"log"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
)

func main() {
	const cname = "eg.c8s.workload"
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
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
