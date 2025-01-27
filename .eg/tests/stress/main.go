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
		shell.Op(shell.New("apt-get install stress").Privileged()),
		shell.Op(shell.Newf("stress -t 15 -c %d", 24)),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
