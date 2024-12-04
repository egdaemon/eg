package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
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
		shell.Newf("tree -a --gitignore %s", egenv.RuntimeDirectory()),
		shell.New("ls -lha /opt/eg/.test/Containerfile"),
		shell.New("ssh -T git@github.com").Lenient(true),
	)
}

func Op1(ctx context.Context, op eg.Op) error {
	log.Println("op1 initiated")
	defer log.Println("op1 completed")
	return shell.Run(
		ctx,
		shell.New("true"),
	)
}

func Op2(ctx context.Context, op eg.Op) error {
	log.Println("op2 initiated")
	defer log.Println("op2 completed")
	return nil
}

func Op3(context.Context, eg.Op) error {
	log.Println("op3 initiated")
	defer log.Println("op3 completed")

	return nil
}

func Op4(context.Context, eg.Op) error {
	log.Println("op4 initiated")
	defer log.Println("op4 completed")
	time.Sleep(1 * time.Second)
	return nil
}

func DaemonTests(ctx context.Context, _ eg.Op) error {
	return eg.Perform(
		ctx,
		eg.Parallel(
			Op1,
			Op2,
		),
		eg.When(env.Boolean(false, "CI"), eg.Sequential(
			Op1,
			Op2,
			Op3,
		)),
	)
}

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	log.Println("main module initiated")
	defer log.Println("main module completed")

	c1 := eg.Container("ubuntu.22.04").BuildFromFile(".test/Containerfile")

	err := eg.Perform(
		ctx,
		// eggit.AutoClone,
		DaemonTests,
		eg.Build(c1),
		eg.Module(ctx, c1, DaemonTests),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
