package main

import (
	"context"
	"log"
	"os"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func Debug(ctx context.Context, op eg.Op) error {
	log.Println("debug initiated")
	defer log.Println("debug completed")
	env.Debug(os.Environ()...)
	return shell.Run(
		ctx,
		shell.New("pwd"),
		shell.New("ls -lha ."),
		shell.New("systemctl status"),
	)
}

func DaemonPostgres(ctx context.Context, _ eg.Op) error {
	daemons := shell.Runtime().Directory("daemons")
	return shell.Run(
		ctx,
		daemons.New("pg_isready").Attempts(15), // 15 attempts = ~3seconds
	)
}

func DaemonTests(ctx context.Context, _ eg.Op) error {
	log.Println("daemon tests")
	daemons := shell.Runtime().Directory("daemons")
	return shell.Run(
		ctx,
		daemons.New("ginkgo run -r -p --randomize-all --randomize-suites --keep-going --output-dir .reports ."),
	)
}

func DaemonLinting(ctx context.Context, _ eg.Op) error {
	log.Println("daemon linting")
	daemons := shell.Runtime().Directory("daemons")
	return shell.Run(
		ctx,
		daemons.New("golangci-lint run -v --timeout 5m"),
	)
}

func ConsoleTests(ctx context.Context, _ eg.Op) error {
	log.Println("console tests")
	return nil
}

func ConsoleLinting(ctx context.Context, _ eg.Op) error {
	log.Println("console linting")
	return nil
}

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	var (
		debcache = egenv.CacheDirectory(".dist")
	)

	if err := os.MkdirAll(debcache, 0700); err != nil {
		log.Fatalln(err)
	}

	c1 := eg.Container("egmeta.ubuntu.22.04")

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Build(
			c1.BuildFromFile(".eg/Containerfile"),
		),
		eg.Parallel(
			eg.Module(ctx, c1, DaemonPostgres, DaemonTests, DaemonLinting),
			eg.Module(ctx, c1, ConsoleTests, ConsoleLinting),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
