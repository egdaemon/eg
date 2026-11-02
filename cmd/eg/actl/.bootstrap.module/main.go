package main

import (
	"context"
	"log"
	"os"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/x/wasi/egerrors"
)

func HelloWorld(ctx context.Context, op eg.Op) error {
	log.Println("hello world initiated")
	defer log.Println("hello world completed")

	e := egenv.Build().FromEnviron(os.Environ()...)

	err := egerrors.Compact(
		e.Unsetenv("EG_GIT_AUTH_ACCESS_TOKEN"),
		e.Unsetenv("GH_TOKEN"),
	)
	if err != nil {
		panic(err)
	}
	env.Debug(egerrors.Zero(e.Environ())...)
	return nil
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		HelloWorld,
	)

	if err != nil {
		log.Fatalln(err)
	}
}
