package main

import (
	"context"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()
	prefix := "p"
	suffix := "s"
	if err := eg.Perform(ctx, eg.Sequential(func(ctx context.Context, op eg.Op) error {
		wrap := func(s string) string {
			return func() string {
				return prefix + s + suffix
			}()
		}
		log.Println(wrap("x"))
		return nil
	})); err != nil {
		log.Fatalln(err)
	}
}
