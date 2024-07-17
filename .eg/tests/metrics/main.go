package main

import (
	"context"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/egmetrics"
	"github.com/egdaemon/eg/runtime/wasi/env"
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
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	log.Println("debug initiated")
	defer log.Println("debug completed")
	env.Debug(os.Environ()...)

	if err := egmetrics.Record(ctx, "cpu", automcpu()); err != nil {
		log.Fatalln(err)
	}
}
