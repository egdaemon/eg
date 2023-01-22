package main

import (
	"context"
	"log"
	"time"

	"github.com/james-lawrence/eg/runtime/wasi/env"
	"github.com/james-lawrence/eg/runtime/wasi/yak"
)

func Op1(context.Context, yak.Op) error {
	log.Println("op1 initiated")
	defer log.Println("op1 completed")

	return nil
}

func Op2(context.Context, yak.Op) error {
	log.Println("op2 initiated")
	defer log.Println("op2 completed")

	return nil
}

func Op3(context.Context, yak.Op) error {
	log.Println("op3 initiated")
	defer log.Println("op3 completed")

	return nil
}

func Op4(context.Context, yak.Op) error {
	log.Println("op4 initiated")
	defer log.Println("op4 completed")
	time.Sleep(1 * time.Second)
	return nil
}

func DaemonTests(ctx context.Context, _ yak.Op) error {
	// c1 := yak.Container("ubuntu.22.04").
	// 	BuildFromFile(".test/Containerfile")

	return yak.Perform(
		ctx,
		yak.Parallel(
			Op1,
			Op2,
			// yak.Module(ctx, c1, Op3),
		),
		yak.When(env.Boolean(false, "CI"), yak.Sequential(
			Op1,
			Op2,
			Op3,
		)),
	)
}

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	c1 := yak.Container("ubuntu.22.04").
		BuildFromFile(".test/Containerfile")

	err := yak.Perform(
		ctx,
		yak.Parallel(
			yak.Module(ctx, c1, Op1),
			yak.Module(ctx, c1, Op2),
			yak.Module(ctx, c1, DaemonTests),
			yak.Module(ctx, c1, Op3),
			yak.Module(ctx, c1, Op4),
		),
	)
	if err != nil {
		panic(err)
	}

}
