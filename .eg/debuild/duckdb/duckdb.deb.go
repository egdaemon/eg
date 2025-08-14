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
	"github.com/egdaemon/eg/runtime/x/wasi/egdebuild"
)

//go:embed .debskel
var debskel embed.FS

const (
	container = "eg.deb.duckdb"
	version   = "1.3.1"
)

var (
	gcfg egdebuild.Config
)

func init() {
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
		egdebuild.Option.Environ("PACKAGE_VERSION", version),
		// egdebuild.Option.Environ("CCACHE_DIR", filepath.Join("src", "build", "ccache")),
		egdebuild.Option.Environ("GIT_COMMIT_HASH", c.Hash.String()),
	)
}

func Prepare(ctx context.Context, o eg.Op) error {
	sruntime := shell.Runtime().Directory(egenv.CacheDirectory())
	return eg.Parallel(
		shell.Op(
			sruntime.Newf("test -d duckdb || git clone -b v%s --depth 1 https://github.com/duckdb/duckdb.git duckdb", version),
			sruntime.New("md5sum duckdb/src/include/duckdb.h"),
			sruntime.New("echo \"6d13054e32644ee436152e1d1c3c8828  duckdb/src/include/duckdb.h\" > duckdb.md5"),
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
	return eg.Sequential(
		eg.Parallel(
			egdebuild.Build(gcfg, egdebuild.Option.Distro("jammy")),
			egdebuild.Build(gcfg, egdebuild.Option.Distro("noble"), egdebuild.Option.NoLint()),
			egdebuild.Build(gcfg, egdebuild.Option.Distro("plucky"), egdebuild.Option.NoLint()),
		),
	)(ctx, o)
}

func Upload(ctx context.Context, o eg.Op) error {
	return egdebuild.UploadDPut(gcfg, errorsx.Must(fs.Sub(debskel, ".debskel")))(ctx, o)
}
