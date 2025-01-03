package debian

import (
	"context"
	"crypto/md5"
	"eg/ci/maintainer"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

const (
	ContainerName = "eg.ubuntu.24.04"
)

func prepare(ctx context.Context, _ eg.Op) error {
	debdir := egenv.EphemeralDirectory(".dist", "deb")
	runtime := shell.Runtime().Privileged().Environ("HOME", "/home/egd")
	return shell.Run(
		ctx,
		runtime.New("git show -s --format=%ct HEAD"),
		runtime.Newf("mkdir -p %s/debian", debdir),
		runtime.Newf("rsync --recursive .dist/deb/.skel/ %s/debian", debdir),
		runtime.Newf("tree -L 2 %s", debdir),
		runtime.Newf("cat .dist/deb/.templates/changelog.tmpl | envsubst | tee %s/debian/changelog", debdir),
		runtime.Newf("cat .dist/deb/.templates/control.tmpl | envsubst | tee %s/debian/control", debdir),
		runtime.Newf("cat .dist/deb/.templates/rules.tmpl | envsubst | tee %s/debian/rules", debdir),
		runtime.Newf("git clone --depth 1 file://${PWD} %s/src", debdir),
	)
}

func build(ctx context.Context, _ eg.Op) error {
	debdir := egenv.EphemeralDirectory(".dist", "deb")
	genv, err := eggolang.Env()
	if err != nil {
		return err
	}

	runtime := shell.Runtime().Privileged().EnvironFrom(genv...)
	return shell.Run(
		ctx,
		runtime.New("go version"),
		runtime.Newf("go -C src build  -tags \"no_duckdb_arrow\" -buildvcs ./cmd/...").Directory(debdir),
		// shell.New("echo ${GPG_PASSPHRASE} | gpg-preset-passphrase --present {key}").Environ("GPG_PASSPHRASE", env.String("", "GPG_PASSPHRASE")),
		runtime.Newf("debuild -S -k%s", maintainer.GPGFingerprint).Directory(debdir),
		runtime.Newf("dput -f -c %s eg eg_${VERSION}_source.changes", egenv.RootDirectory(".dist", "deb", "dput.config")).Directory(filepath.Dir(debdir)),
	)
}

func Builder(name string, distro string) eg.ContainerRunner {
	c := eggit.EnvCommit()

	return eg.Container(name).
		OptionEnv("VCS_REVISION", c.Hash.String()).
		OptionEnv("VERSION", fmt.Sprintf("0.0.%d", time.Now().Add(dynamicduration(10*time.Second, distro)).UnixMilli())).
		OptionEnv("DEBEMAIL", maintainer.Email).
		OptionEnv("DEBFULLNAME", maintainer.Name).
		OptionEnv("DISTRO", distro).
		OptionEnv("CHANGELOG_DATE", c.Committer.When.Format(time.RFC1123Z))
}

func Build(ctx context.Context, _ eg.Op) error {
	return eg.Perform(ctx, prepare, build)
}

// uint64 to prevent negative values
func dynamichashversion(i string, n uint64) uint64 {
	digest := md5.Sum([]byte(i))
	return binary.LittleEndian.Uint64(digest[:]) % n
}

// generate a *consistent* duration based on the input i within the
// provided window. this isn't the best location for these functions.
// but the lack of a better location.
func dynamicduration(window time.Duration, i string) time.Duration {
	if window == 0 {
		return 0
	}

	return time.Duration(dynamichashversion(i, uint64(window)))
}
