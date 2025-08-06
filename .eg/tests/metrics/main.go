package main

import (
	"context"
	"log"
	"math/rand"

	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egmetrics"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoverage"
)

type MetricCPU struct {
	Load float32
}

func automcpu() MetricCPU {
	return MetricCPU{
		Load: rand.Float32(),
	}
}

func autocoverage() *events.Coverage {
	return &events.Coverage{
		Path:       "foo",
		Statements: 10,
		Branches:   10,
	}
}

func modulemetric(ctx context.Context, o eg.Op) error {
	log.Println("metric initiated")
	defer log.Println("metric completed")
	return egmetrics.Record(ctx, "cpu", automcpu())
}

func modulecoverage(ctx context.Context, o eg.Op) error {
	log.Println("coverage initiated")
	defer log.Println("coverage completed")
	return fficoverage.Report(ctx, autocoverage())
}

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	defer log.Println("metrics/coverage test completed")

	if err := egmetrics.Record(ctx, "cpu", automcpu()); err != nil {
		log.Fatalln(err)
	}

	if err := fficoverage.Report(ctx, autocoverage()); err != nil {
		log.Fatalln(err)
	}

	err := eg.Perform(
		ctx,
		eg.Build(eg.DefaultModule()),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			eg.Sequential(
				modulemetric,
				modulecoverage,
			),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
