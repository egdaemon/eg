package archlinux

import (
	"context"
	"eg/ci/maintainer"
	"fmt"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

const (
	ContainerName = "eg.arch.latest"
)

func Builder(name string) eg.ContainerRunner {
	return eg.Container(name).
		OptionVolume(
			".eg/.temp", "/opt/eg/.build",
		)
}

func Build(ctx context.Context, _ eg.Op) error {
	golang := shell.Runtime().
		Environ("GOCACHE", egenv.CacheDirectory("golang", "build")).
		Environ("GOMODCACHE", egenv.CacheDirectory("golang", "mod")).
		Environ("PKGDEST", filepath.Join(os.TempDir(), "pacman")).
		Environ("PACKAGER", fmt.Sprintf("%s <%s>", maintainer.Name, maintainer.Email))

	return shell.Run(
		ctx,
		golang.New("tree -a --gitignore /opt/eg/.dist/archlinux"),
		golang.New("rsync --recursive /opt/eg/.dist/archlinux/ .build"),
		golang.New("chown -R build:build .build"),
		golang.New("tls -lha /cache"),
		golang.Directory(".build").New("sudo -E -u build makepkg -f -c -C"),
		golang.New("tree -a --gitignore .build"),
		golang.Newf(
			"rsync --recursive %s/ %s",
			filepath.Join(os.TempDir(), "pacman"),
			egenv.CacheDirectory("pacman"),
		),
	)
}
