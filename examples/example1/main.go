package main

import (
	"errors"
	"log"

	"github.com/james-lawrence/eg/runtime/wasi/env"
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

	return errors.New("derp")
}

func Op4(yak.Op) error {
	return nil
}

func main() {
	log.Println("DERP", env.Boolean(false, "EG_CI", "CI"))
	yak.Perform(
		yak.Parallel(
			Op1,
			Op2,
		),
		yak.When(
			yak.Sequential(
				Op1,
				Op2,
				Op3,
			),
			env.Boolean(false, "EG_CI", "CI"),
		),
	)
}
