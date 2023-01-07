package main

import (
	"context"
	"log"
	"os"

	"github.com/james-lawrence/eg/runtime/wasi/env"
	"github.com/james-lawrence/eg/runtime/wasi/langx"
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

func DaemonTests() error {
	return yak.Perform(
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
	if err := yak.Perform(yak.Module(DaemonTests)); err != nil {
		panic(err)
	}

	if i, err := os.Stat("/bin/bash"); err == nil {
		log.Println("found bin directory", i.Name())
	} else {
		log.Println("unable to locate /bin/bash", err)
	}

	if i, err := os.Stat("/usr/bin/bash"); err == nil {
		log.Println("found bash", i.Name())
	} else {
		log.Println("unable to locate /usr/bin/bash", err)
	}

	if err := shell.Run(context.Background(), "echo hello world"); err != nil {
		panic(err)
	}

	yak.Container("ubuntu.22.04").
		Definition(langx.Must(os.Open("Containerfile"))).
		Perform(yak.Module(DaemonTests))
}
