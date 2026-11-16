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
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
	"github.com/egdaemon/eg/runtime/x/wasi/egdebuild"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
	"github.com/egdaemon/eg/runtime/x/wasi/eggpg"
)

const (
	container = "eg.ubuntu"
)

//go:embed .debskel
var debskel embed.FS

var (
	gcfg egdebuild.Config
)

func gpgoptions() []eggpg.Option {
	// eggpg.Options() takes GNUPGHOME from the environment, which inside the container is the
	// host-provided /eg.mnt/.gnupg (not writable by us: "unsafe ownership", "Permission denied").
	// pin the isolated per-container home instead.
	return eggpg.Options().IgnoreLocalGNU().Home("/home/egd/.gnupg")
}

func init() {
	c := eggit.EnvCommit()
	gcfg = egdebuild.New(
		"eg",
		"",
		egenv.DirectoryWorking(),
		egdebuild.Option.Maintainer(maintainer.Name, maintainer.Email),
		egdebuild.Option.SigningKeyID(maintainer.GPGFingerprint),
		egdebuild.Option.ChangeLogDate(c.Committer.When),
		egdebuild.Option.Description("eg", "eg a developer friendly, self hostable ci/cd platform"),
		egdebuild.Option.Version("0.0.:autopatch:"),
		egdebuild.Option.Debian(errorsx.Must(fs.Sub(debskel, ".debskel"))),
		egdebuild.Option.DependsBuild("golang-1.27", "dh-make", "debhelper", "duckdb", "libc6-dev (>= 2.35)", "libbtrfs-dev", "libassuan-dev", "libdevmapper-dev", "libglib2.0-dev", "libgpgme-dev", "libgpg-error-dev", "libprotobuf-dev", "libprotobuf-c-dev", "libseccomp-dev", "libselinux1-dev", "libsystemd-dev"),
		egdebuild.Option.Depends(
			"podman",       // container runtime
			"netavark",     // network management
			"aardvark-dns", // dns management for containers
			"duckdb",       // metrics support
			"bindfs",       // mapping filesystem permissions
			"acl",          // ??
			"pciutils",     // required for GPU access
		),
		egdebuild.Option.Envvar("VCS_REVISION", c.Hash.String()),
		egdebuild.Option.Envvar("VCS_TIMESTAMP", c.Committer.When.UTC().Format(time.RFC3339)),
		egdebuild.Option.Environ(eggpg.Env(gpgoptions()...)...),
	)
}

func Prepare(ctx context.Context, o eg.Op) error {
	return egdebuild.Prepare(Runner(), errorsx.Must(fs.Sub(debskel, ".debskel")))(ctx, o)
}

// container for this package.
func Runner() eg.C8S {
	return eg.Container(container)
}

func Build(ctx context.Context, o eg.Op) error {
	return eg.Sequential(
		// override eg compute local's setting of the home directory.
		// ideally we should default to this except when performing local
		// compute workloads.
		egbug.DebugFailure(
			eggpg.Seed(gpgoptions()...),
			eggpg.Debug(gpgoptions()...),
		),
		egdebuild.Build(gcfg, egdebuild.Option.Distro(egdebuild.UbuntuLatestCodename)),
		egdebuild.Build(
			gcfg,
			egdebuild.Option.Distro(egdebuild.UbuntuLatestCodename),
			egdebuild.Option.BuildBinary(20*time.Minute),
			egdebuild.Option.Environ(eggolang.Env()...),
			egdebuild.Option.Environ(eggpg.Env(gpgoptions()...)...),
			egdebuild.Option.NoLint(),
		),
	)(ctx, o)
}

// Copy the built deb packages into the workload's .dist directory.
func Copy(ctx context.Context, o eg.Op) error {
	return egdebuild.Copy(gcfg, egenv.DirectoryWorkload(".dist"))(ctx, o)
}

func Upload(ctx context.Context, o eg.Op) error {
	return egdebuild.UploadDPut(gcfg, errorsx.Must(fs.Sub(debskel, ".debskel")), egdebuild.Option.Timeout(20*time.Minute))(ctx, o)
}
