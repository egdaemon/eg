package archlinux

import (
	"context"
	"eg/compute/maintainer"
	"fmt"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

const (
	ContainerName = "eg.arch.latest"
)

func Builder(name string) eg.ContainerRunner {
	return eg.Container(name)
}

func Build(ctx context.Context, _ eg.Op) error {
	cdir := egenv.CacheDirectory(".dist", "pacman")
	templatedir := egenv.WorkingDirectory(".dist", "archlinux")
	runtime := eggolang.Runtime().
		Environ("PKGDEST", cdir).
		Environ("BUILDDIR", filepath.Join("/", "tmp", "build")).
		Environ("SRCDEST", filepath.Join("/", "tmp", "src")).
		Environ("PACKAGER", fmt.Sprintf("%s <%s>", maintainer.Name, maintainer.Email))

	return shell.Run(
		ctx,
		runtime.Newf("mkdir -p %s", cdir),
		runtime.New("pwd; ls -lha .; makepkg -f").Directory(templatedir),
		runtime.Newf("paccache -c %s -rk2", cdir),
	)
}
