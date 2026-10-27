package eggolang_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

func ExampleAutoTest() {
	// run the tests for every module in the working directory with the defaults.
	err := eg.Perform(
		context.Background(),
		eggolang.AutoTest(),
		eggolang.RecordCoverage,
	)
	if err != nil {
		log.Fatalln(err)
	}
}

func ExampleAutoInstall() {
	// go install ./... for every module in the working directory.
	err := eg.Perform(
		context.Background(),
		eggolang.AutoInstall(),
	)
	if err != nil {
		log.Fatalln(err)
	}
}

func ExampleAutoCompile() {
	// go build ./... for every module in the working directory.
	err := eg.Perform(
		context.Background(),
		eggolang.AutoCompile(),
	)
	if err != nil {
		log.Fatalln(err)
	}
}

func ExampleNew() {
	// options are chained and finished by the command to run.
	err := eg.Perform(
		context.Background(),
		eggolang.New().Timeout(10*time.Minute).Tags("integration").AutoInstall(),
		eggolang.New().Timeout(10*time.Minute).Randomize(0).Verbose(true).NoCache().AutoTest(),
		eggolang.New().WorkingDirectory("subproject").Flags("-race").AutoCompile(),
	)
	if err != nil {
		log.Fatalln(err)
	}
}

func ExampleEnv() {
	os.Setenv("CACHE_DIRECTORY", "/cache")
	os.Setenv(_eg.EnvComputeCacheDirectory, "/cache")

	// the process environment is inherited, the cache directories are set last so they take precedence.
	effective := envx.NewEnvironFromStrings(eggolang.Env()...)

	fmt.Println("GOCACHE", effective.String("", "GOCACHE"))
	fmt.Println("GOMODCACHE", effective.String("", "GOMODCACHE"))

	// Output:
	// GOCACHE /cache/.eg/golang/build
	// GOMODCACHE /cache/.eg/golang/mod
}
