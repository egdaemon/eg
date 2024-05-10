package debian

import (
	"context"
	"eg/ci/maintainer"
	"fmt"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
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
	return shell.Run(
		ctx,
		shell.New("env").
			Environ("GOPROXY", "off").
			Environ("GOWORK", "off"),
		shell.New("/usr/lib/go-1.22/bin/go version"),
		shell.New("/usr/lib/go-1.22/bin/go build -buildvcs ./cmd/...").
			Directory(".dist/deb/src"),
		// shell.New("echo ${GPG_PASSPHRASE} | gpg-preset-passphrase --present 1472F4128AD327A04323220509F9FEB7D4D09CF4").Environ("GPG_PASSPHRASE", env.String("", "GPG_PASSPHRASE")),
		shell.New("debuild -S -k1472F4128AD327A04323220509F9FEB7D4D09CF4").Directory(".dist/deb"),
		shell.New("dput -f -c deb/dput.config eg eg_${VERSION}_source.changes").Directory(".dist"),
	)
}

func Builder(name string, ts time.Time, distro string) eg.ContainerRunner {
	return eg.Container(name).
		OptionEnv("VCS_REVISION", egenv.GitCommit()).
		OptionEnv("VERSION", fmt.Sprintf("0.0.%d", ts.Unix())).
		OptionEnv("DEBEMAIL", maintainer.Email).
		OptionEnv("DEBFULLNAME", maintainer.Name).
		OptionEnv("DISTRO", distro).
		OptionEnv("CHANGELOG_DATE", ts.Format(time.RFC1123Z)).
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
