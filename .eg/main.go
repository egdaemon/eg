package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func Debug(ctx context.Context, op eg.Op) error {
	log.Println("debug initiated")
	defer log.Println("debug completed")
	env.Debug(os.Environ()...)
	return shell.Run(
		ctx,
		shell.New("ls -lha /opt/eg"),
		shell.New("ls -lha /root"),
		shell.New("ls -lha /root/.ssh && md5sum /root/.ssh/known_hosts"),
		shell.New("ssh -T git@github.com || true"),
	)
}

func PrepareDebian(ctx context.Context, _ eg.Op) error {
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

func BuildDebian(ctx context.Context, _ eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("env").
			Environ("GOPROXY", "off").
			Environ("GOWORK", "off"),
		shell.New("/usr/lib/go-1.22/bin/go version"),
		shell.New("/usr/lib/go-1.22/bin/go build -buildvcs ./cmd/...").
			Directory(".dist/deb/src"),
		shell.New("echo ${GPG_PASSPHRASE} | gpg-preset-passphrase --present 1472F4128AD327A04323220509F9FEB7D4D09CF4").Environ("GPG_PASSPHRASE", env.String("", "GPG_PASSPHRASE")),
		shell.New("debuild -S -k1472F4128AD327A04323220509F9FEB7D4D09CF4").Directory(".dist/deb"),
		shell.New("/usr/bin/false"),
		shell.New("dput -f -c deb/dput.config eg eg_${VERSION}_source.changes").Directory(".dist"),
	)
}

func ubuntucontainer(name string, ts time.Time, distro string) eg.ContainerRunner {
	return eg.Container(name).
		OptionEnv("VCS_REVISION", egenv.GitCommit()).
		OptionEnv("VERSION", fmt.Sprintf("0.0.%d", ts.Unix())).
		OptionEnv("DEBEMAIL", "engineering@egdaemon.com").
		OptionEnv("DEBFULLNAME", "engineering").
		OptionEnv("DISTRO", distro).
		OptionEnv("CHANGELOG_DATE", ts.Format(time.RFC1123Z)).
		OptionVolumeWritable(
			".eg/.cache/.dist", "/opt/eg/.dist",
		).
		OptionVolume(
			".dist/deb", "/opt/eg/.dist/deb",
		)
}

// main defines the setup for the CI process. here is where you define all
// of the environments and tasks you wish to run.
func main() {
	const (
		ubuntuname = "eg.ubuntu.22.04"
	)
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	ts := time.Now()
	jammy := ubuntucontainer(ubuntuname, ts, "jammy")
	noble := ubuntucontainer(ubuntuname, ts.Add(time.Second), "noble")

	err := eg.Perform(
		ctx,
		Debug,
		eggit.AutoClone,
		eg.Parallel(
			eg.Build(eg.Container(ubuntuname).BuildFromFile(".dist/Containerfile")),
		),
		eg.Parallel(
			eg.Module(ctx, jammy, eg.Sequential(PrepareDebian, BuildDebian)),
			eg.Module(ctx, noble, eg.Sequential(PrepareDebian, BuildDebian)),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
