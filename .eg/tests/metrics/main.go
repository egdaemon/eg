package main

import (
	"context"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egmetrics"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

type MetricCPU struct {
	Load float32
}

func automcpu() MetricCPU {
	return MetricCPU{
		Load: rand.Float32(),
	}
}

func Debug(ctx context.Context, op eg.Op) error {
	log.Println("debug initiated")
	defer log.Println("debug completed")
	env.Debug(os.Environ()...)
	return shell.Run(
		ctx,
	)
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

	err := eg.Perform(
		ctx,
		Debug,
	)

	if err != nil {
		log.Fatalln(err)
	}

	time.Sleep(time.Second)
}
