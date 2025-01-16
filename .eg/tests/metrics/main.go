package main

import (
	"context"
	"log"
	"math/rand"

	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egmetrics"
)

type MetricCPU struct {
	Load float32
}

func automcpu() MetricCPU {
	return MetricCPU{
		Load: rand.Float32(),
	}
}

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	if err := egmetrics.Record(ctx, "cpu", automcpu()); err != nil {
		log.Fatalln(err)
	}
}
