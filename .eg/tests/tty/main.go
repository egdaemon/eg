package main

import (
	"context"
	"log"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		shell.Op(shell.New("systemctl status systemd-resolved.service").Privileged().Timeout(time.Second)),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
