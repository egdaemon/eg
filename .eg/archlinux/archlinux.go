package archlinux

import (
	"context"
	"eg/compute/maintainer"
	"fmt"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egccache"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

const (
	ContainerName = "eg.arch.latest"
)

func Builder(name string) eg.ContainerRunner {
	return eg.Container(name)
}

func Build(ctx context.Context, o eg.Op) error {
	cdir := egenv.CacheDirectory(".dist", "pacman")
	templatedir := egenv.WorkingDirectory(".dist", "archlinux")

	runtime := shell.Runtime().
		EnvironFrom(eggolang.Env()...).
		EnvironFrom(egccache.Env()...).
		Environ("PKGDEST", cdir).
		Environ("BUILDDIR", filepath.Join("/", "tmp", "build")).
		Environ("SRCDEST", filepath.Join("/", "tmp", "src")).
		Environ("PACKAGER", fmt.Sprintf("%s <%s>", maintainer.Name, maintainer.Email))

	return eg.Sequential(
		egccache.PrintStatistics,
		shell.Op(
			runtime.Newf("mkdir -p %s", cdir),
			runtime.New("pwd; ls -lha .; makepkg -f").Directory(templatedir),
			runtime.New("git checkout -- .").Directory(templatedir), // reset PKGBUILD modifications caused by pkgver()
			runtime.Newf("paccache -c %s -rk2", cdir),
		),
		egccache.PrintStatistics,
	)(ctx, o)
}
