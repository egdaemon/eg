package main

import (
	"context"
	"log"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
)

func Op1(ctx context.Context, op eg.Op) error {
	log.Println("op1 initiated")
	defer log.Println("op1 completed")
	return nil
}

func Op2(context.Context, eg.Op) error {
	log.Println("op2 initiated")
	defer log.Println("op2 completed")

	return nil
}

func Op3(context.Context, eg.Op) error {
	log.Println("op3 initiated")
	defer log.Println("op3 completed")

	return nil
}

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	log.Println("MODULE m2.mb")
	err := eg.Perform(
		ctx,
		Op1,
		Op2,
		Op3,
	)
	if err != nil {
		log.Fatalln(err)
	}
}
