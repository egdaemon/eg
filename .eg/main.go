package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/james-lawrence/eg/runtime/wasi/egenv"
	"github.com/james-lawrence/eg/runtime/wasi/shell"
	"github.com/james-lawrence/eg/runtime/wasi/yak"
)

func PrepareDebian(ctx context.Context, _ yak.Op) error {
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

func BuildDebian(ctx context.Context, _ yak.Op) error {
	return shell.Run(
		ctx,
		shell.New("/usr/lib/go-1.21/bin/go build -mod=vendor ./cmd/...").Environ("GOPROXY", "off").Directory(".dist/deb/src"),
		shell.New("debuild -S -k1472F4128AD327A04323220509F9FEB7D4D09CF4").Directory(".dist/deb"),
		shell.New("dput -f -c deb/dput.config eg eg_${VERSION}_source.changes").Directory(".dist"),
	)
}

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	var (
		debcache = egenv.CachePath(".dist")
	)

	if err := os.MkdirAll(debcache, 0700); err != nil {
		log.Fatalln(err)
	}

	c1 := yak.Container("eg.ubuntu.22.04").
		OptionEnv("VERSION", fmt.Sprintf("0.0.%d", time.Now().Unix())).
		OptionEnv("DEBEMAIL", "jljatone@gmail.com").
		OptionEnv("DEBFULLNAME", "James Lawrence").
		OptionEnv("DISTRO", "jammy").
		OptionEnv("CHANGELOG_DATE", time.Now().Format(time.RFC1123Z)).
		OptionVolumeWritable(
			".eg/.cache/.dist", "/opt/eg/.dist",
		).
		OptionVolume(
			".dist/deb", "/opt/eg/.dist/deb",
		)

	err := yak.Perform(
		ctx,
		yak.Parallel(
			yak.Build(yak.Container("eg.ubuntu.22.04").
				BuildFromFile(".dist/Containerfile")),
			yak.Build(yak.Container("eg.debian.build").
				BuildFromFile(".dist/deb/Containerfile")),
		),
		yak.Module(ctx, c1, PrepareDebian, BuildDebian),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
