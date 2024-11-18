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
	cdir := egenv.CacheDirectory(".dist", "pacman")
	templatedir := egenv.RootDirectory(".dist", "archlinux")
	bdir := egenv.EphemeralDirectory(".build")
	pkgdest := egenv.EphemeralDirectory("pacman")
	golang := shell.Runtime().
		Environ("PKGDEST", pkgdest).
		Environ("PACKAGER", fmt.Sprintf("%s <%s>", maintainer.Name, maintainer.Email))

	return shell.Run(
		ctx,
		golang.Newf("echo %s", templatedir),
		golang.Newf("sudo -E -u build rsync --recursive %s/ %s", templatedir, bdir),
		golang.Newf("chown -R build:root %s", bdir),
		golang.Newf("sudo -u build mkdir -p %s", pkgdest),
		golang.Directory(bdir).New("sudo --preserve-env=PKGDEST,PACKAGER -u build makepkg -f -c -C"),
		golang.Newf("rsync --recursive %s/ %s", pkgdest, cdir),
		golang.Newf("paccache -c %s -rk2", cdir),
	)
}
