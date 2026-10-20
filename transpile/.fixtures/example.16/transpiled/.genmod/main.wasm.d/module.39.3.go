package main

import (
	"context"
	"log"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

const constant = "c"

var global = "g"

type Pair[T any] struct{ A, B T }

func Max[T int | float64](a, b T) T {
	if a > b {
		return a
	}
	return b
}
func init() {
	log.Println("init")
}
func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()
	local := global + "!"
	if err := eg.Perform(ctx, eg.Sequential(func(ctx context.Context, op eg.Op) error {
		log.Println(global, local, constant, Pair[int]{1, 2}, Max(1, 2))
		return nil
	})); err != nil {
		log.Fatalln(err)
	}
}
