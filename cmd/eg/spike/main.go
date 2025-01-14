package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/userx"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithCancelCause(context.Background())
	go cmdopts.Cleanup(ctx, done, &sync.WaitGroup{}, func() {
		log.Println("waiting for systems to shutdown")
	}, os.Kill, os.Interrupt)

	log.Println("DERP DERP", userx.DefaultRuntimeDirectory())
}
