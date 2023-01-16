package main

import (
	"context"
	"log"
	"time"

	"github.com/james-lawrence/eg/runtime/wasi/env"
	derpyak "github.com/james-lawrence/eg/runtime/wasi/yak"
)

func Op1(context.Context, derpyak.Op) error {
	log.Println("op1 initiated")
	defer log.Println("op1 completed")

	return nil
}

func Op2(context.Context, derpyak.Op) error {
	log.Println("op2 initiated")
	defer log.Println("op2 completed")

	return nil
}

func Op3(context.Context, derpyak.Op) error {
	log.Println("op3 initiated")
	defer log.Println("op3 completed")

	return nil
}

func Op4(context.Context, derpyak.Op) error {
	log.Println("op4 initiated")
	defer log.Println("op4 completed")

	return nil
}

func DaemonTests(ctx context.Context, _ derpyak.Op) error {
	return derpyak.Perform(
		ctx,
		derpyak.Parallel(
			Op1,
			Op2,
		),
		derpyak.When(env.Boolean(false, "EG_CI", "CI"), derpyak.Sequential(
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

	c1 := derpyak.Container("ubuntu.22.04").
		BuildFromFile(".test/Containerfile")

	if err := derpyak.Module(ctx, c1, DaemonTests, Op4); err != nil {
		panic(err)
	}
}
