package archlinux

import (
	"context"
	"eg/ci/maintainer"
	"fmt"
	"path/filepath"

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
	pkgdest := filepath.Join("/", "tmp", "pacman")
	mkpkgruntime := shell.Runtime().
		Environ("PKGDEST", pkgdest).
		Environ("BUILDDIR", filepath.Join("/", "tmp", "build")).
		Environ("SRCDEST", filepath.Join("/", "tmp", "src")).
		Environ("PACKAGER", fmt.Sprintf("%s <%s>", maintainer.Name, maintainer.Email))

	return shell.Run(
		ctx,
		mkpkgruntime.Newf("mkdir -p %s", cdir),
		mkpkgruntime.New("sudo --preserve-env=PKGDEST,PACKAGER,BUILDDIR -u build env"),
		mkpkgruntime.Directory(templatedir).New("sudo --preserve-env=PKGDEST,PACKAGER,BUILDDIR,SRCDEST -g root -u build pwd"),
		mkpkgruntime.Directory(templatedir).New("sudo --preserve-env=PKGDEST,PACKAGER,BUILDDIR,SRCDEST -g root -u build makepkg -f"),
		mkpkgruntime.Newf("rsync --recursive %s/ %s", pkgdest, cdir),
		mkpkgruntime.Newf("paccache -c %s -rk2", cdir),
	)
}
