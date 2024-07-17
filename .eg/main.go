package main

import (
	"context"
	"eg/ci/archlinux"
	"eg/ci/debian"
	"log"
	"os"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/shellx"
)

func Debug(ctx context.Context, op eg.Op) error {
	log.Println("debug initiated")
	defer log.Println("debug completed")
	env.Debug(os.Environ()...)
	return shell.Run(
		ctx,
		// shell.New("ls -lha /opt/eg"),
		// shell.New("ls -lha /root"),
		// shell.New("ls -lha /root/.ssh && md5sum /root/.ssh/known_hosts"),
		// shell.New("ssh -T git@github.com || true"),
	)
}

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	if s, err := shellx.String(ctx, "/usr/bin/echo", "hello world"); err == nil {
		log.Println("shell output", s)
	}

	err := eg.Perform(
		ctx,
		// Debug,
		eggit.AutoClone,
		eg.Parallel(
			eg.Build(eg.Container(debian.ContainerName).BuildFromFile(".dist/deb/Containerfile")),
			eg.Build(eg.Container(archlinux.ContainerName).BuildFromFile(".dist/archlinux/Containerfile")),
		),
		eg.Parallel(
			eg.Module(ctx, debian.Builder(debian.ContainerName, "jammy"), debian.Build),
			eg.Module(ctx, debian.Builder(debian.ContainerName, "noble"), debian.Build),
			// eg.Module(ctx, archlinux.Builder(archlinux.ContainerName), archlinux.Build),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
