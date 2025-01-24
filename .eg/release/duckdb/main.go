package main

import (
	"context"
	"log"

	"eg/compute/debuild/duckdb"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		duckdb.Prepare,
		eg.Module(
			ctx,
			duckdb.Runner(),
			duckdb.Build,
			duckdb.Upload,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
