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
)

const (
	email = "engineering@egdaemon.com"
)

func Debug(ctx context.Context, op eg.Op) error {
	log.Println("debug initiated")
	defer log.Println("debug completed")
	env.Debug(os.Environ()...)
	return shell.Run(
		ctx,
		shell.New("ls -lha /opt/eg"),
		shell.New("ls -lha /root"),
		shell.New("ls -lha /root/.ssh && md5sum /root/.ssh/known_hosts"),
		shell.New("ssh -T git@github.com || true"),
	)
}

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	const (
		ubuntuname = "eg.ubuntu.22.04"
	)

	os.Setenv("EMAIL", "engineering@egdaemon.com")

	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	ts := time.Now()
	jammy := debian.Builder(debian.ContainerName, ts, "jammy")
	noble := debian.Builder(debian.ContainerName, ts.Add(time.Second), "noble")
	arch := archlinux.Builder(archlinux.ContainerName)

	err := eg.Perform(
		ctx,
		// Debug,
		eggit.AutoClone,
		eg.Parallel(
			eg.Build(eg.Container(ubuntuname).BuildFromFile(".dist/Containerfile")),
			eg.Build(eg.Container(archlinux.ContainerName).BuildFromFile(".dist/archlinux/Containerfile")),
		),
		eg.Parallel(
			eg.Module(ctx, jammy, debian.Build),
			eg.Module(ctx, noble, debian.Build),
			eg.Module(ctx, arch, archlinux.Build),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
