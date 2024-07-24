package archlinux

import (
	"context"
	"eg/ci/maintainer"
	"fmt"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

const (
	ContainerName = "eg.arch.latest"
)

func Builder(name string) eg.ContainerRunner {
	return eg.Container(name)
}

func Build(ctx context.Context, _ eg.Op) error {
	gocache := egenv.CacheDirectory("golang", "pkg")
	gomodcache := egenv.CacheDirectory("golang", "mod")
	bdir := egenv.EphemeralDirectory(".build")
	pkgdest := egenv.CacheDirectory("pacman")
	golang := shell.Runtime().
		Environ("GOCACHE", gocache).
		Environ("GOMODCACHE", gomodcache).
		Environ("PKGDEST", pkgdest).
		Environ("PACKAGER", fmt.Sprintf("%s <%s>", maintainer.Name, maintainer.Email))

	return shell.Run(
		ctx,
		golang.Newf("mkdir -p %s %s", bdir, gocache),
		golang.Newf("chmod 0777 %s %s %s", gocache, pkgdest, egenv.EphemeralDirectory()),
		golang.Newf("rsync --recursive /opt/eg/.dist/archlinux/ %s", bdir),
		golang.Newf("chown -R build:root %s", bdir),
		golang.Directory(bdir).New("sudo -E -u build makepkg -f -c -C"),
		golang.Newf("paccache -c %s -rk1", pkgdest),
	)
}
