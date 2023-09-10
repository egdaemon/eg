package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/james-lawrence/eg/runtime/wasi/egenv"
	"github.com/james-lawrence/eg/runtime/wasi/langx"
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
		shell.New("git show -s --format=%ct HEAD"),
		shell.New("rm -rf .dist/deb/debian/* && mkdir -p .dist/deb/debian"),
		shell.New("rsync --recursive .dist/deb/.skel/ .dist/deb/debian"),
		shell.New("cat .dist/deb/.templates/changelog.tmpl | envsubst | tee .dist/deb/debian/changelog"),
		shell.New("cat .dist/deb/debian/changelog"),
		shell.New("cat .dist/deb/.templates/control.tmpl | envsubst | tee .dist/deb/debian/control"),
		shell.New("cat .dist/deb/.templates/rules.tmpl | envsubst | tee .dist/deb/debian/rules"),
	)
}

func BuildDebian(ctx context.Context, _ yak.Op) error {
	return shell.Run(
		ctx,
		shell.New("cd .dist/deb && debuild -S -k1472F4128AD327A04323220509F9FEB7D4D09CF4"),
		shell.New("cd .dist && dput -f -c deb/dput.config eg eg_${VERSION}_source.changes"),
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
		OptionVolume(
			filepath.Join(langx.Must(os.UserHomeDir()), ".gnupg"), filepath.Join("/", "root", ".gnupg"),
		).
		OptionVolumeWritable(
			".eg/.cache/.dist", "/opt/eg/.dist",
		).
		OptionVolume(
			".dist/deb", "/opt/eg/.dist/deb",
		)

	err := yak.Perform(
		ctx,
		yak.Parallel(
			BuildContainer(yak.Container("eg.ubuntu.22.04").
				BuildFromFile(".dist/Containerfile")),
			BuildContainer(yak.Container("eg.debian.build").
				BuildFromFile(".dist/deb/Containerfile")),
		),
		yak.Module(ctx, c1, PrepareDebian, BuildDebian),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
