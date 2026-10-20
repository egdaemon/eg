// captured values providing operations through method values.
package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

type Runner struct {
	name string
}

func (r Runner) Run(ctx context.Context, op eg.Op) error {
	log.Println(r.name)
	return nil
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	r := Runner{name: "r"}

	err := eg.Perform(
		ctx,
		eg.Module(
			ctx,
			eg.DefaultModule(),
			r.Run,
			eg.Sequential(r.Run),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
