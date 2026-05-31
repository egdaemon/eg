// Package egworkload builds a debian package that configures a container for running eg workloads.
package egworkload

import (
	"context"
	"embed"
	"io/fs"
	"time"

	"eg/compute/errorsx"
	"eg/compute/maintainer"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/x/wasi/egdebuild"
)

//go:embed .debskel
var debskel embed.FS

var (
	gcfg egdebuild.Config
)

func init() {
	c := eggit.EnvCommit()
	gcfg = egdebuild.New(
		"egworkload",
		"",
		egenv.WorkingDirectory(".eg", "debuild", "egworkload", "rootfs"),
		egdebuild.Option.Maintainer(maintainer.Name, maintainer.Email),
		egdebuild.Option.SigningKeyID(maintainer.GPGFingerprint),
		egdebuild.Option.ChangeLogDate(c.Committer.When),
		egdebuild.Option.Version("0.0.:autopatch:"),
		egdebuild.Option.DependsBuild("rsync", "curl", "tree", "software-properties-common", "ca-certificates"),

		egdebuild.Option.Debian(errorsx.Must(fs.Sub(debskel, ".debskel"))),
		egdebuild.Option.Depends(
			"egbootstrap",
			"eg",
			"sudo",
			"golang",
			"systemd-container", // required for machinectl to be present for use within shell.New(...) commands. which allows invoking systemctl --user commands.
		),
		egdebuild.Option.Description(
			"configures a container for running eg workloads",
			"installs and configures all components required to run eg workloads in a container",
		),
	)
}

func Prepare(ctx context.Context, o eg.Op) error {
	return eg.Parallel(
		egdebuild.Prepare(Runner(), nil),
	)(ctx, o)
}

func Runner() eg.ContainerRunner {
	return egdebuild.Runner()
}

func Build(ctx context.Context, o eg.Op) error {
	return egdebuild.Build(gcfg,
		egdebuild.Option.Distro(egdebuild.UbuntuLatestCodename),
	)(ctx, o)
}

func Upload(ctx context.Context, o eg.Op) error {
	return egdebuild.UploadDPut(gcfg, errorsx.Must(fs.Sub(debskel, ".debskel")), egdebuild.Option.Timeout(20*time.Minute))(ctx, o)
}
