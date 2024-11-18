package duckdb

import (
	"context"
	"crypto/md5"
	"eg/ci/maintainer"
	"embed"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
	"github.com/shirou/gopsutil/v4/cpu"
)

//go:embed .debskel
var debskel embed.FS

func prepare(ctx context.Context, _ eg.Op) error {
	relpath, _ := filepath.Rel(egenv.RootDirectory(), egenv.RuntimeDirectory())
	count, _ := cpu.Counts(false)
	log.Println("-------------------------------------------", "BDIR", relpath, runtime.NumCPU(), count, "-------------------------------------------")

	sruntime := shell.Runtime().
		Directory(relpath)

	return shell.Run(
		ctx,
		sruntime.New("git clone git@github.com:duckdb/duckdb duckdb"),
		// TODO: get cores working properly.
		// sruntime.Newf("echo make -C duckdb -j %d bundle-library", runtime.NumCPU()),
		// sruntime.Newf("make -C duckdb -j %d bundle-library", runtime.NumCPU()),
	)
}

func build(distro string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		pdir, err := os.MkdirTemp(egenv.RuntimeDirectory(), "duckdb.deb.build.*")
		if err != nil {
			return err
		}

		bdir := filepath.Join(pdir, "duckdb")
		if err := os.MkdirAll(bdir, 0755); err != nil {
			return err
		}

		if err = egfs.CloneFS(ctx, bdir, ".debskel", debskel); err != nil {
			return err
		}

		relpath, _ := filepath.Rel(egenv.RootDirectory(), bdir)

		c := eggit.EnvCommit()
		runtime := shell.Runtime().
			Directory(relpath).
			Environ("DEB_PACKAGE_NAME", "duckdb").
			Environ("PACKAGE_VERSION", "1.1.3").
			Environ("DEB_VERSION", fmt.Sprintf("1.1.3.%d", c.Committer.When.Add(dynamicduration(10*time.Second, distro)).UnixMilli())).
			Environ("DEB_DISTRO", distro).
			Environ("DEB_CHANGELOG_DATE", c.Committer.When.Format(time.RFC1123Z)).
			Environ("DEB_EMAIL", maintainer.Email).
			Environ("DEB_FULLNAME", maintainer.Name)

		return shell.Run(
			ctx,
			runtime.New("rsync --verbose --progress --recursive --perms ../../duckdb/ src/"),
			runtime.New("cat debian/changelog | envsubst | tee debian/changelog"),
			runtime.New("cat debian/control | envsubst | tee debian/control"),
			runtime.New("cat debian/rules | envsubst | tee debian/rules"),
			runtime.New("cat debian/changelog"),
			runtime.Newf("debuild -S -k%s", maintainer.GPGFingerprint),
			runtime.New("dput -d -f -P -c dput.config duckdb ../duckdb_${DEB_VERSION}_source.changes"),
		)
	}
}

func Build() eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		env.Debug(os.Environ()...)
		return eg.Perform(
			ctx,
			prepare,
			// build("jammy"),
			build("noble"),
			build("ocular"),
		)
	}
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
