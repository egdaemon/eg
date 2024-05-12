package debian

import (
	"context"
	"eg/ci/maintainer"
	"fmt"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

const (
	ContainerName = "eg.ubuntu.22.04"
)

func prepare(ctx context.Context, _ eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("git show -s --format=%ct HEAD"),
		shell.New("rm -rf .dist/deb/debian/* && mkdir -p .dist/deb/debian"),
		shell.New("rsync --recursive .dist/deb/.skel/ .dist/deb/debian"),
		shell.New("cat .dist/deb/.templates/changelog.tmpl | envsubst | tee .dist/deb/debian/changelog"),
		shell.New("cat .dist/deb/.templates/control.tmpl | envsubst | tee .dist/deb/debian/control"),
		shell.New("cat .dist/deb/.templates/rules.tmpl | envsubst | tee .dist/deb/debian/rules"),
		shell.New("git clone --depth 1 file://${PWD} ${PWD}/.dist/deb/src"),
	)
}

func build(ctx context.Context, _ eg.Op) error {
	runtime := shell.Runtime()
	return shell.Run(
		ctx,
		runtime.New("env").
			Environ("GOPROXY", "off").
			Environ("GOWORK", "off"),
		runtime.New("/usr/lib/go-1.22/bin/go version"),
		runtime.New("/usr/lib/go-1.22/bin/go build -buildvcs ./cmd/...").
			Directory(".dist/deb/src"),
		// shell.New("echo ${GPG_PASSPHRASE} | gpg-preset-passphrase --present {key}").Environ("GPG_PASSPHRASE", env.String("", "GPG_PASSPHRASE")),
		runtime.Newf("debuild -S -k%s", maintainer.GPGFingerprint).Directory(".dist/deb"),
		runtime.New("dput -f -c deb/dput.config eg eg_${VERSION}_source.changes").Directory(".dist"),
	)
}

func Builder(name string, distro string) eg.ContainerRunner {
	c := eggit.EnvCommit()

	return eg.Container(name).
		OptionEnv("VCS_REVISION", c.Hash.String()).
		OptionEnv("VERSION", fmt.Sprintf("0.0.%d", c.Committer.When.Unix())).
		OptionEnv("DEBEMAIL", maintainer.Email).
		OptionEnv("DEBFULLNAME", maintainer.Name).
		OptionEnv("DISTRO", distro).
		OptionEnv("CHANGELOG_DATE", c.Committer.When.Format(time.RFC1123Z)).
		OptionVolumeWritable(
			".eg/.cache/.dist", "/opt/eg/.dist",
		).
		OptionVolume(
			".dist/deb", "/opt/eg/.dist/deb",
		)
}

func Build(ctx context.Context, _ eg.Op) error {
	return eg.Perform(ctx, prepare, build)
}
