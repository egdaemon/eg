package main

import (
	"context"
	"log"
	"math/rand"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egmetrics"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
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

	return shell.Run(
		ctx,
		shell.Newf("truncate --size 0 %s", egenv.RuntimeDirectory("environ.env")).Lenient(true),
		shell.Newf("apt-get install stress").Privileged(),
		shell.Newf("stress -t 5 -c %d", 24),
	)
}

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		Debug,
		egbug.Users,
		eg.When(true, Debug),
		eg.When(true, egbug.Users),
	)

	if err != nil {
		log.Fatalln(err)
	}

	if err := egmetrics.Record(ctx, "cpu", automcpu()); err != nil {
		log.Fatalln(err)
	}
}
