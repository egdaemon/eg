// Package egbootstrap builds a debian with base system settings for eg workloads
package egbootstrap

import (
	"context"
	"embed"

	"github.com/egdaemon/eg/.eg/maintainer"

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
		"egbootstrap",
		"",
		egenv.WorkingDirectory(".eg", "debuild", "egbootstrap"),
		egdebuild.Option.Maintainer(maintainer.Name, maintainer.Email),
		egdebuild.Option.ChangeLogDate(c.Committer.When),
		egdebuild.Option.Version("0.0.:autopatch:"),
		egdebuild.Option.Debian(debskel),
	)
}

func Prepare(ctx context.Context, o eg.Op) error {
	return eg.Parallel(
		egdebuild.Prepare,
	)(ctx, o)
}

func Build() eg.OpFn {
	return eg.Parallel(
		egdebuild.Build(gcfg, egdebuild.Option.Distro("jammy")),
		egdebuild.Build(gcfg, egdebuild.Option.Distro("noble")),
		egdebuild.Build(gcfg, egdebuild.Option.Distro("oracular")),
	)
}
