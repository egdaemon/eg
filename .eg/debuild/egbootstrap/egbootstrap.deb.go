// Package egbootstrap builds a debian with base system settings for eg compute systems.
// primarily provides basic configuration settings like available package repositories
// and system configuration.
package egbootstrap

import (
	"context"
	"embed"
	"io/fs"

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

// Returns the latest release codename for ubuntu
func UbuntuLatestCodename() string {
	return "questing"
}

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
		egdebuild.Option.DependsBuild("rsync", "curl", "tree", "software-properties-common", "ca-certificates"),
		egdebuild.Option.Depends(
			"software-properties-common",
			"systemd-container", // required for machinectl to be present for use within shell.New(...) commands. which allows invoking systemctl --user commands.
		),
		egdebuild.Option.Description(
			"configures the machine for running as a eg module",
			"performs various changes to the system for running as an eg module. makes egd a privileged user and adds scripts for setting up apt repositories",
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
	return eg.Parallel(
		egdebuild.Build(gcfg, egdebuild.Option.Distro("jammy")),
		egdebuild.Build(gcfg, egdebuild.Option.Distro("noble"), egdebuild.Option.NoLint()),
		egdebuild.Build(gcfg, egdebuild.Option.Distro("questing"), egdebuild.Option.NoLint()),
	)(ctx, o)
}

func Upload(ctx context.Context, o eg.Op) error {
	return egdebuild.UploadDPut(gcfg, errorsx.Must(fs.Sub(debskel, ".debskel")))(ctx, o)
}
