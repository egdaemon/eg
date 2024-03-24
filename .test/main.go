package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/wasi/yak"
)

func Debug(ctx context.Context, op yak.Op) error {
	log.Println("debug initiated")
	defer log.Println("debug completed")
	env.Debug(os.Environ()...)
	return shell.Run(
		ctx,
		shell.New("ls -lha /opt/eg"),
		shell.New("ssh -v -T git@github.com"),
	)
}

func Op1(ctx context.Context, op yak.Op) error {
	log.Println("op1 initiated")
	defer log.Println("op1 completed")
	return shell.Run(
		ctx,
		shell.New("true").Environ("USER", "root"),
	)
}

func Op2(ctx context.Context, op yak.Op) error {
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
	return yak.Perform(
		ctx,
		yak.Parallel(
			Debug,
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

func BuildContainer(r yak.Runner) yak.OpFn {
	return func(ctx context.Context, _ yak.Op) error {
		return r.CompileWith(ctx)
	}
}

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	log.Println("main module initiated")
	defer log.Println("main module completed")

	if environ, err := env.FromPath("/opt/egruntime/environ"); err == nil {
		env.Debug(environ...)
	} else {
		log.Println("DERP DERP", err)
	}

	// c1 := yak.Container("ubuntu.22.04").BuildFromFile(string(langx.Must(fs.ReadFile(embedded, "Containerfile"))))
	c1 := yak.Container("ubuntu.22.04").BuildFromFile(".test/Containerfile")
	// c1 := yak.Container("ubuntu.22.04").PullFrom("ubuntu:jammy")

	err := yak.Perform(
		ctx,
		Debug,
		eggit.AutoClone,
		yak.Build(c1),
		yak.Module(ctx, c1, DaemonTests),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
