package main

import (
	"context"
	"eg/ci/debbuild/duckdb"
	"log"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	if err := eg.Perform(ctx, duckdb.Build()); err != nil {
		log.Fatalln(err)
	}
}
