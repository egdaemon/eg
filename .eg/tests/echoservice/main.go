// Package echoservice run a
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

	c1 := eg.DefaultModule()
	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		shell.Op(
			shell.New("apt-get install -y iptables").Privileged(),
		),
		eg.Build(c1),
		eg.Module(
			ctx,
			c1.OptionLiteral("--publish", "3000"),
			shell.Op(
				shell.New("apt-get install -y nmap socat").Privileged(),
				shell.New("echo -----------------------------------------"),
				shell.New("socat -v tcp-l:3000,fork exec:'/bin/cat'").Timeout(egenv.TTL()),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
