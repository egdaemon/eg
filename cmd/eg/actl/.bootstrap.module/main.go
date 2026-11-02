package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/env"
)

func HelloWorld(ctx context.Context, op eg.Op) error {
	log.Println("hello world initiated")
	defer log.Println("hello world completed")

	e := egenv.Build().FromEnviron(os.Environ()...)

	err := Compact(
		e.Unsetenv("EG_GIT_AUTH_ACCESS_TOKEN"),
		e.Unsetenv("GH_TOKEN"),
	)
	if err != nil {
		panic(err)
	}
	env.Debug(Zero(e.Environ())...)
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

// Compact returns the first error in the set, if any.
func Compact(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

// Zero logs that the error occurred but otherwise ignores it.
func Zero[T any](v T, err error) T {
	if err == nil {
		return v
	}

	if cause := log.Output(2, fmt.Sprintln(err)); cause != nil {
		panic(cause)
	}

	return v
}
