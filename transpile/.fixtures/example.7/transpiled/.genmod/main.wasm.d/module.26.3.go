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
	type config struct {
		Name  string
		Ports []int
	}
	type alias = map[string]config
	cfg := config{Name: "example", Ports: []int{80, 443}}
	lookup := alias{"example": cfg}
	if err := eg.Perform(ctx, eg.Sequential(func(ctx context.Context, op eg.Op) error {
		log.Println(lookup["example"].Name)
		return nil
	})); err != nil {
		log.Fatalln(err)
	}
}
