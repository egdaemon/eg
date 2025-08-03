package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

func Debug(runtime shell.Command) eg.OpFn {
	return eg.Sequential(
		egbug.Log("---------------------------- failed inspection initiated ----------------------------"),
		shell.Op(
			runtime.New("env"),
			runtime.New("id"),
		),
		egbug.Log("---------------------------- failed inspection completed ----------------------------"),
	)
}

// test case for baremetal.
func Test(ctx context.Context, op eg.Op) error {
	return eg.Sequential(
		// ensure a stable environment.
		egbug.EnsureEnv("d706bb599e81b24b903cf034d724b7e3", egbug.EnsureEnviron()...),
	)(ctx, op)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Build(eg.DefaultModule()),
		egbug.DebugFailure(
			// ensure that the user isnt egd
			shell.Op(
				shell.New("test $(id -nu) != egd"),
				shell.New("test $(id -ng) != egd"),
			),
			Debug(shell.Runtime()),
		),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			eg.Sequential(
				Test,
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
