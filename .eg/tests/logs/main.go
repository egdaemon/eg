package main

import (
	"context"
	"log"
	"time"
	"uuid"

	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"golang.org/x/time/rate"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()
	l := rate.NewLimiter(rate.Every(time.Second), 1)

	for l.Wait(ctx) == nil {
		log.Println(uuid.NewV7().String())
	}
}
