package duckdb

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"eg/compute/errorsx"
	"eg/compute/maintainer"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egccache"
	"github.com/egdaemon/eg/runtime/x/wasi/egdebuild"
)

//go:embed .debskel
var debskel embed.FS

const (
	container = "eg.deb.duckdb"
	version   = "1.4.1"
)

var (
	gcfg egdebuild.Config
)

func init() {
	egccache.CacheDirectory()
	c := eggit.EnvCommit()
	gcfg = egdebuild.New(
		"duckdb",
		"",
		egenv.CacheDirectory("duckdb"),
		egdebuild.Option.Maintainer(maintainer.Name, maintainer.Email),
		egdebuild.Option.SigningKeyID(maintainer.GPGFingerprint),
		egdebuild.Option.ChangeLogDate(c.Committer.When),
		egdebuild.Option.Version(fmt.Sprintf("%s.:autopatch:", version)),
		egdebuild.Option.Description("duckdb", "embeddable columnar database"),
		egdebuild.Option.Debian(errorsx.Must(fs.Sub(debskel, ".debskel"))),
		egdebuild.Option.DependsBuild("rsync", "curl", "tree", "ca-certificates", "cmake", "ninja-build", "libssl-dev", "git"),
		egdebuild.Option.Envvar("PACKAGE_VERSION", version),
		egdebuild.Option.Envvar("GIT_COMMIT_HASH", c.Hash.String()),
	)
}

func Prepare(ctx context.Context, o eg.Op) error {
	sruntime := shell.Runtime().Directory(egenv.CacheDirectory())
	return eg.Parallel(
		shell.Op(
			sruntime.Newf("test -d duckdb || git clone -b v%s --depth 1 https://github.com/duckdb/duckdb.git duckdb", version),
			sruntime.New("md5sum duckdb/src/include/duckdb.h"),
			sruntime.New("echo \"2a20d340931922b25919dd8a870365a9  duckdb/src/include/duckdb.h\" > duckdb.md5"),
			sruntime.New("md5sum -c duckdb.md5"),
		),
		egdebuild.Prepare(Runner(), errorsx.Must(fs.Sub(debskel, ".debskel"))),
	)(ctx, o)
}

// container for this package.
func Runner() eg.ContainerRunner {
	return eg.Container(container)
}

func Build(ctx context.Context, o eg.Op) error {
	const latest = "questing"
	return eg.Sequential(
		eg.Parallel(
			// build the package to improve the chances it'll actually build in within ubuntu launchpad.
			// egdebuild.Build(
			// 	gcfg,
			// 	egdebuild.Option.Distro(latest),
			// 	egdebuild.Option.BuildBinary(20*time.Minute),
			// 	egdebuild.Option.Environ(egccache.Env()...),
			// 	egdebuild.Option.NoLint(),
			// ),
			egdebuild.Build(gcfg, egdebuild.Option.Distro(latest), egdebuild.Option.NoLint()),
			egdebuild.Build(gcfg, egdebuild.Option.Distro("noble"), egdebuild.Option.NoLint()),
			egdebuild.Build(gcfg, egdebuild.Option.Distro("jammy")),
		),
	)(ctx, o)
}

func Upload(ctx context.Context, o eg.Op) error {
	return egdebuild.UploadDPut(gcfg, errorsx.Must(fs.Sub(debskel, ".debskel")))(ctx, o)
}
