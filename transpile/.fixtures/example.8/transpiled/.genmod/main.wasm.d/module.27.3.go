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
	greet := func(name string) string {
		return "hello " + name
	}
	var fib func(int) int
	fib = func(n int) int {
		if n < 2 {
			return n
		}
		return fib(n-1) + fib(n-2)
	}
	if err := eg.Perform(ctx, eg.Sequential(func(ctx context.Context, op eg.Op) error {
		log.Println(greet("world"), fib(10))
		return nil
	})); err != nil {
		log.Fatalln(err)
	}
}
