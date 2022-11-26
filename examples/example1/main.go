package main

import (
	"errors"
	"log"

	"github.com/james-lawrence/eg/runtime/wasi/env"
	. "github.com/james-lawrence/eg/runtime/wasi/yak"
)

func Op1(Op) error {
	log.Println("op1 initiated")
	defer log.Println("op1 completed")

	return nil
}

func Op2(Op) error {
	log.Println("op2 initiated")
	defer log.Println("op2 completed")

	return nil
}

func Op3(Op) error {
	log.Println("op3 initiated")
	defer log.Println("op3 completed")

	return errors.New("derp")
}

func Op4(Op) error {
	return nil
}

func main() {
	Perform(
		Parallel(
			Op1,
			Op2,
		),
		When(env.Boolean(false, "EG_CI", "CI"), Sequential(
			Op1,
			Op2,
			Op3,
		)),
	)
}
