package main

import (
	"context"
	"log"
	"time"

	"github.com/james-lawrence/eg/runtime/wasi/env"
	"github.com/james-lawrence/eg/runtime/wasi/shell"
	"github.com/james-lawrence/eg/runtime/wasi/yak"
)

func Op1(yak.Op) error {
	log.Println("op1 initiated")
	defer log.Println("op1 completed")

	return nil
}

func Op2(yak.Op) error {
	log.Println("op2 initiated")
	defer log.Println("op2 completed")

	return nil
}

func Op3(yak.Op) error {
	log.Println("op3 initiated")
	defer log.Println("op3 completed")

	return nil
}

func Op4(yak.Op) error {
	return nil
}

func PrintDir(yak.Op) error {
	return shell.Run(context.Background(), "echo hello world")
}

func DaemonTests(ctx context.Context) error {
	return yak.Perform(
		ctx,
		yak.Parallel(
			Op1,
			Op2,
		),
		yak.When(env.Boolean(false, "EG_CI", "CI"), yak.Sequential(
			Op1,
			Op2,
			Op3,
		)),
	)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	if err := shell.Run(ctx, "echo hello world"); err != nil {
		panic(err)
	}

	if err := shell.Run(ctx, "ls -lha .test"); err != nil {
		panic(err)
	}

	err := yak.Container("ubuntu.22.04").
		DefinitionFile(".test/Containerfile").
		Perform(ctx, yak.Module(DaemonTests))
	if err != nil {
		panic(err)
	}
}
