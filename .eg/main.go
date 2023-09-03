package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/runtime/wasi/shell"
	"github.com/james-lawrence/eg/runtime/wasi/yak"
)

func BuildContainer(r yak.ContainerRunner) yak.OpFn {
	return func(ctx context.Context, _ yak.Op) error {
		return r.CompileWith(ctx)
	}
}

func PrepareDebian(ctx context.Context, _ yak.Op) error {
	return shell.Run(
		ctx,
		shell.New("rm -rf .dist/deb/debian && mkdir -p .dist/deb/debian"),
		shell.New("rsync --recursive .dist/deb/.skel/ .dist/deb/debian"),
		shell.New("cat .dist/deb/.templates/changelog.tmpl | VERSION=\"0.0.1\" DISTRO=\"jammy\" DEBEMAIL=\"jljatone@gmail.com\" CHANGELOG_DATE=\"$(date +\"%a, %d %b %Y %T %z\")\" envsubst | tee .dist/deb/debian/changelog"),
		shell.New("cat .dist/deb/debian/changelog"),
		shell.New("cat .dist/deb/.templates/control.tmpl | envsubst | tee .dist/deb/debian/control"),
		shell.New("cat .dist/deb/.templates/rules.tmpl | envsubst | tee .dist/deb/debian/rules"),
	)
}

func BuildDebian(ctx context.Context, _ yak.Op) error {
	return shell.Run(
		ctx,
		shell.New("ls -lha").Directory(".dist/deb"),
		shell.New("cd .dist/deb && debuild -S"),
	)
}

func Debug(ctx context.Context, _ yak.Op) error {
	for _, en := range os.Environ() {
		log.Println(en)
	}

	return shell.Run(
		ctx,
		shell.New("podman --version"),
	)
}

func DebugGPG(ctx context.Context, _ yak.Op) error {
	return shell.Run(
		ctx,
		shell.New("ls -lha /root/.gnupg/")
		shell.New("gpg --list-keys"),
		shell.New("ls -lha /tmp"),
		shell.New("env && gpgconf --list-dirs agent-socket").Environ("GPG_AGENT_INFO", "/root/.gnupg/socket"),
	)
}

func DebianBuild(ctx context.Context, o yak.Op) error {
	return yak.Sequential(
		yak.Parallel(
			Debug,
			PrepareDebian,
		),
		BuildDebian,
	)(ctx, o)
}

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	// c1 := yak.Container("eg.ubuntu.22.04").PullFrom("quay.io/podman/stable")

	// -v $(HOME)/.gnupg:/opt/bw/.dist/cache/.gnupg

	c1 := yak.Container("eg.ubuntu.22.04").
		OptionPrivileged().
		OptionVolumeWithPermissions(
			filepath.Join("/", "home", "james.lawrence", ".gnupg"), filepath.Join("/", "root", ".gnupg"), "rw",
		).
		OptionVolumeWithPermissions(
			envx.String("", "GPG_AGENT_INFO"), "/root/.gnupg/socket", "rw",
		)

	err := yak.Perform(
		ctx,
		yak.Parallel(
			BuildContainer(yak.Container("eg.ubuntu.22.04").
				BuildFromFile(".dist/Containerfile")),
			BuildContainer(yak.Container("eg.debian.build").
				BuildFromFile(".dist/deb/Containerfile")),
		),
		yak.Parallel(
			yak.Module(ctx, c1, DebugGPG, DebianBuild),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
}
