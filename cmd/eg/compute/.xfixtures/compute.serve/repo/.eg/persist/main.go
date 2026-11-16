// Package main commits one file and leaves another uncommitted, the changes serve has to persist to the session branch.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		shell.Op(
			shell.New("echo two > tracked.txt && git add tracked.txt && git -c user.name=Ada -c user.email=ada@example.com commit -qm 'committed by the workload'"),
			shell.New("echo uncommitted > uncommitted.txt"),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
