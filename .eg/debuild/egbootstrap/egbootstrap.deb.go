// Package egbootstrap builds a debian with base system settings for eg workloads
package egbootstrap

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"

	"eg/compute/errorsx"
	"eg/compute/maintainer"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egdebuild"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

//go:embed .debskel
var debskel embed.FS

var (
	gcfg egdebuild.Config
)

func init() {
	c := eggit.EnvCommit()
	gcfg = egdebuild.New(
		"egbootstrap",
		"",
		egenv.WorkingDirectory(".eg", "debuild", "egbootstrap", "rootfs"),
		egdebuild.Option.Maintainer(maintainer.Name, maintainer.Email),
		egdebuild.Option.SigningKeyID(maintainer.GPGFingerprint),
		egdebuild.Option.ChangeLogDate(c.Committer.When),
		egdebuild.Option.Version("0.0.:autopatch:"),
		egdebuild.Option.Debian(errorsx.Must(fs.Sub(debskel, ".debskel"))),
		egdebuild.Option.DependsBuild("rsync", "curl", "tree", "software-properties-common", "build-essential", "ca-certificates"),
		egdebuild.Option.Depends("software-properties-common"),
	)
}

func Prepare(ctx context.Context, o eg.Op) error {
	return eg.Parallel(
		egdebuild.Prepare,
	)(ctx, o)
}

func Runner() eg.ContainerRunner {
	return egdebuild.Runner()
}

func Build(ctx context.Context, o eg.Op) error {
	return eg.Parallel(
		egdebuild.Build(gcfg, egdebuild.Option.Distro("jammy")),
		egdebuild.Build(gcfg, egdebuild.Option.Distro("noble")),
		egdebuild.Build(gcfg, egdebuild.Option.Distro("oracular")),
	)(ctx, o)
}

func Upload(ctx context.Context, o eg.Op) error {
	if err := egfs.CloneFS(ctx, egenv.EphemeralDirectory(), filepath.Join("dput.config"), errorsx.Must(fs.Sub(debskel, ".debskel"))); err != nil {
		return err
	}
	root := fmt.Sprintf("deb.%s", gcfg.Name)
	bdir := egenv.EphemeralDirectory(root)
	runtime := egdebuild.Runtime(gcfg)
	return shell.Run(
		ctx,
		runtime.New("ls *.tar.xz | xargs -I {} tar -tvf {}").Directory(bdir),
		runtime.Newf("ls %s/*_source.changes | xargs -I {} dput -f -c %s %s {}", bdir, egenv.EphemeralDirectory("dput.config"), gcfg.Name).Privileged(),
	)
}
