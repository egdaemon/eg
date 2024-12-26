package main

import (
	"context"
	"log"
	"math/rand"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egmetrics"
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

	return shell.Run(
		ctx,
		shell.New("env"),
		shell.New("pwd"),
		shell.Newf("truncate --size 0 %s", egenv.RuntimeDirectory("environ.env")).Lenient(true),
		shell.Newf("tree -L 2 %s", egenv.RootDirectory()),
		shell.Newf("apt-get install stress"),
		shell.Newf("stress -t 5 -c %d", 24),
		// shell.Newf("stress -t 5 -m %d", 24), // requires 6GB of ram
	)
}

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		Debug,
	)

	if err != nil {
		log.Fatalln(err)
	}

	if err := egmetrics.Record(ctx, "cpu", automcpu()); err != nil {
		log.Fatalln(err)
	}
}
