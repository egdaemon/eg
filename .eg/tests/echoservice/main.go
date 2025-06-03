// Package echoservice run a webservice locally
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		shell.Op(
			shell.New("echo -----------------------------------------"),
			shell.New("env"),
			shell.Newf("socat -v tcp-l:%d,fork exec:'/bin/cat'", egenv.Int(3000, "EG_COMPUTE_PORT_0")).Timeout(egenv.TTL()),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
