package main

import (
	"context"
	"log"
	"strings"
	"time"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	egalias "github.com/egdaemon/eg/runtime/wasi/eg"
	envalias "github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), envalias.TTL())
	defer done()
	upper := strings.ToUpper("value")
	wait := 2 * time.Second
	if err := egalias.Perform(ctx, egalias.Sequential(func(ctx context.Context, op egalias.Op) error {
		log.Println(upper, wait)
		return nil
	})); err != nil {
		log.Fatalln(err)
	}
}
