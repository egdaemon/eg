package eg

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
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egdebuild"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

const (
	container = "eg.ubuntu.24.04"
)

//go:embed .debskel
var debskel embed.FS

var (
	gcfg egdebuild.Config
)

func init() {
	c := eggit.EnvCommit()
	gcfg = egdebuild.New(
		"eg",
		"",
		egenv.CacheDirectory(".dist", "eg"),
		egdebuild.Option.Maintainer(maintainer.Name, maintainer.Email),
		// egdebuild.Option.BuildCommand(func(cfg *egdebuild.Config, runtime shell.Command) shell.Command {
		// 	return runtime.Newf("debuild --no-lintian -us -uc -S -k%s", cfg.SignatureKeyID).Privileged()
		// }),
		// egdebuild.Option.NoLint(),
		egdebuild.Option.SigningKeyID(maintainer.GPGFingerprint),
		egdebuild.Option.ChangeLogDate(c.Committer.When),
		egdebuild.Option.Version("0.0.:autopatch:"),
		egdebuild.Option.Debian(errorsx.Must(fs.Sub(debskel, ".debskel"))),
		egdebuild.Option.DependsBuild("golang-1.25", "dh-make", "debhelper", "duckdb", "libc6-dev (>= 2.35)", "libbtrfs-dev", "libassuan-dev", "libdevmapper-dev", "libglib2.0-dev", "libgpgme-dev", "libgpg-error-dev", "libprotobuf-dev", "libprotobuf-c-dev", "libseccomp-dev", "libselinux1-dev", "libsystemd-dev"),
		egdebuild.Option.Depends("podman", "duckdb", "bindfs", "acl"),
		egdebuild.Option.Envvar("VCS_REVISION", c.Hash.String()),
	)
}

func Prepare(ctx context.Context, o eg.Op) error {
	debdir := egenv.CacheDirectory(".dist", "eg")
	sruntime := shell.Runtime()
	return eg.Sequential(
		shell.Op(
			sruntime.Newf("git -C \"%s\" pull --rebase -X theirs file://${PWD}/ || (rm -rf %s && git clone --depth 1 file://${PWD}/ %s)", debdir, debdir, debdir),
		),
		egdebuild.Prepare(Runner(), errorsx.Must(fs.Sub(debskel, ".debskel"))),
	)(ctx, o)
}

// container for this package.
func Runner() eg.ContainerRunner {
	return eg.Container(container)
}

func Build(ctx context.Context, o eg.Op) error {
	return eg.Parallel(
		egdebuild.Build(gcfg, egdebuild.Option.Distro("jammy")),
		egdebuild.Build(gcfg, egdebuild.Option.Distro("noble"), egdebuild.Option.NoLint()),
		egdebuild.Build(gcfg, egdebuild.Option.Distro("plucky"), egdebuild.Option.NoLint()),
		egdebuild.Build(
			gcfg,
			egdebuild.Option.Distro("plucky"),
			egdebuild.Option.BuildBinary(20*time.Minute),
			egdebuild.Option.Environ(eggolang.Env()...),
			egdebuild.Option.NoLint(),
		),
	)(ctx, o)
}

func Upload(ctx context.Context, o eg.Op) error {
	return egdebuild.UploadDPut(gcfg, errorsx.Must(fs.Sub(debskel, ".debskel")))(ctx, o)
}
