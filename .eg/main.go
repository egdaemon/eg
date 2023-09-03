package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/james-lawrence/eg/runtime/wasi/shell"
	"github.com/james-lawrence/eg/runtime/wasi/yak"
)

func BuildContainer(r yak.ContainerRunner) yak.OpFn {
	return func(ctx context.Context, _ yak.Op) error {
		return r.CompileWith(ctx)
	}
}

func PrepareContainerMounts(ctx context.Context, _ yak.Op) error {
	return shell.Run(
		ctx,
		shell.New("echo hello world 1").Timeout(10*time.Second),
		shell.New("echo hello world 2").Timeout(10*time.Second),
	)
}

func Debug(ctx context.Context, _ yak.Op) error {
	for _, en := range os.Environ() {
		log.Println(en)
	}

	return shell.Run(
		ctx,
		shell.New("podman --version").Timeout(10*time.Second),
	)
}

func DebianBuild(ctx context.Context, o yak.Op) error {
	return yak.Sequential(
		yak.Parallel(
			Debug,
			// BuildDebContainer,
			// PrepareContainerMounts,
		),
	)(ctx, o)
}

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	// c1 := yak.Container("eg.ubuntu.22.04").PullFrom("quay.io/podman/stable")

	err := yak.Perform(
		ctx,
		yak.Parallel(
			BuildContainer(yak.Container("eg.ubuntu.22.04").
				BuildFromFile(".dist/Containerfile")),
			BuildContainer(yak.Container("eg.debian.build").
				BuildFromFile(".dist/deb/Containerfile")),
		),
		yak.Parallel(
			yak.Module(ctx, yak.Container("eg.ubuntu.22.04").OptionPrivileged(), DebianBuild),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
