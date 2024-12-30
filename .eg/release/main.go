package main

import (
	"context"
	"eg/ci/archlinux"
	"eg/ci/debian"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/execx"
)

func Prepare(ctx context.Context, op eg.Op) error {
	return shell.Run(
		ctx,
		shell.Newf("pwd"),
		// shell.Newf("mkdir -p %s", egenv.CacheDirectory(".dist")),
		shell.New("mkdir -p .eg/.cache/.dist"),
	)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	if s, err := execx.String(ctx, "/usr/bin/echo", "hello world"); err == nil {
		log.Println("shell output", s)
	}

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		Prepare,
		eg.Parallel(
			eg.Build(eg.Container(debian.ContainerName).BuildFromFile(".dist/deb/Containerfile")),
			eg.Build(eg.Container(archlinux.ContainerName).BuildFromFile(".dist/archlinux/Containerfile")),
		),
		eg.Parallel(
			eg.Module(ctx, debian.Builder(debian.ContainerName, "jammy"), debian.Build),
			eg.Module(ctx, debian.Builder(debian.ContainerName, "noble"), debian.Build),
			eg.Module(ctx, debian.Builder(debian.ContainerName, "oracular"), debian.Build),
			// eg.Module(ctx, archlinux.Builder(archlinux.ContainerName), archlinux.Build),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
