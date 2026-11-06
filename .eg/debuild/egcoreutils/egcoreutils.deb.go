package egcoreutils

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"eg/compute/errorsx"
	"eg/compute/maintainer"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egcargo"
	"github.com/egdaemon/eg/runtime/x/wasi/egdebuild"
)

//go:embed .debskel
var debskel embed.FS

const (
	// git@github.com:uutils/coreutils.git
	container = "eg.deb.egcoreutils"
	version   = "0.12.0"                                   // uutils release tag.
	commit    = "dc1efd89948a9ca4c78c3a4b9a6ac891019a8c69" // commit of the release tag.
	toolchain = "1.93.1"                                   // rust toolchain, at least the 1.88 uutils requires.
	target    = "wasm32-wasip1"
)

var gcfg egdebuild.Config

func init() {
	c := eggit.EnvCommit()
	gcfg = egdebuild.New(
		"egcoreutils",
		"",
		egenv.DirectoryCache("egcoreutils", "dist"),
		egdebuild.Option.Maintainer(maintainer.Name, maintainer.Email),
		egdebuild.Option.SigningKeyID(maintainer.GPGFingerprint),
		egdebuild.Option.ChangeLogDate(c.Committer.When),
		egdebuild.Option.Version(fmt.Sprintf("%s.:autopatch:", version)),
		egdebuild.Option.Description("egcoreutils", "uutils coreutils compiled to WebAssembly (wasm32-wasip1) for eg's guest shell"),
		egdebuild.Option.Debian(errorsx.Must(fs.Sub(debskel, ".debskel"))),
		egdebuild.Option.Architecture("all"),
		egdebuild.Option.Runtime(func(c shell.Command) shell.Command {
			return c.MaybePrivileged(true) // temporary until we register the gpg key.
			// this allows the runner to access the .gnupg directory we mount via compute local
		}),
	)
}

func Prepare(ctx context.Context, o eg.Op) error {
	sruntime := shell.Runtime().Directory(egenv.DirectoryCache())
	return eg.Parallel(
		shell.Op(
			sruntime.Newf("test -d uutils || git clone -b %s --depth 1 https://github.com/uutils/coreutils.git uutils", version),
			sruntime.Newf("test \"$(git -C uutils rev-parse HEAD)\" = %s", commit),
		),
		egdebuild.Prepare(Runner(), errorsx.Must(fs.Sub(debskel, ".debskel"))),
	)(ctx, o)
}

// container for this package.
func Runner() eg.ContainerRunner {
	return eg.Container(container)
}

// Compile builds coreutils.wasm into the package's source directory. launchpad's rust has no wasm32 standard
// library, so the package ships the module built here instead of building it from source.
func Compile(ctx context.Context, o eg.Op) error {
	runtime := egcargo.Runtime().
		Environ("RUSTUP_HOME", egcargo.CacheDirectory("rustup")).
		Directory(egenv.DirectoryCache("uutils"))
	dist := egenv.DirectoryCache("egcoreutils", "dist")
	return shell.Run(
		ctx,
		runtime.Newf("rustup toolchain install %s --profile minimal --target %s", toolchain, target),
		runtime.Newf("rustup run %s cargo build --release --locked --target %s --no-default-features --features feat_wasm", toolchain, target).Timeout(30*time.Minute),
		runtime.Newf("install -D -m 0644 target/%s/release/coreutils.wasm %s/coreutils.wasm", target, dist),
		runtime.Newf("install -m 0644 LICENSE %s/LICENSE", dist),
	)
}

func Build(ctx context.Context, o eg.Op) error {
	return eg.Sequential(
		Compile,
		egdebuild.Build(gcfg, egdebuild.Option.Distro(egdebuild.UbuntuLatestCodename), egdebuild.Option.NoLint()),
	)(ctx, o)
}

func Upload(ctx context.Context, o eg.Op) error {
	return egdebuild.UploadDPut(gcfg, errorsx.Must(fs.Sub(debskel, ".debskel")), egdebuild.Option.Timeout(20*time.Minute))(ctx, o)
}
