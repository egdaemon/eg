package duckdb

import (
	"context"
	"eg/ci/maintainer"
	"embed"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

//go:embed .debskel
var debskel embed.FS

func prepare(ctx context.Context, _ eg.Op) error {
	relpath, _ := filepath.Rel(egenv.RootDirectory(), egenv.RuntimeDirectory())
	log.Println("BDIR", relpath)

	runtime := shell.Runtime().
		Directory(relpath)

	return shell.Run(
		ctx,
		runtime.New("tree ."),
		runtime.New("git clone git@github.com:duckdb/duckdb duckdb"),
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
		log.Println("BDIR", bdir, relpath)
		c := eggit.EnvCommit()
		runtime := shell.Runtime().
			Directory(relpath).
			Environ("DEB_PACKAGE_NAME", "duckdb").
			Environ("DEB_VERSION", "1.0.1").
			Environ("DEB_DISTRO", distro).
			Environ("DEB_CHANGELOG_DATE", c.Committer.When.Format(time.RFC1123Z)).
			Environ("DEB_EMAIL", maintainer.Email).
			Environ("DEB_FULLNAME", maintainer.Name)

		return shell.Run(
			ctx,
			runtime.New("rsync --verbose --progress --recursive --perms ../../duckdb/ src/"),
			runtime.New("tree -L 2 ."),
			runtime.New("cat debian/changelog | envsubst | tee debian/changelog"),
			runtime.New("cat debian/control | envsubst | tee debian/control"),
			runtime.New("cat debian/rules | envsubst | tee debian/rules"),
			runtime.New("cat debian/changelog"),
			runtime.Newf("debuild -S -k%s", maintainer.GPGFingerprint),
			runtime.New("pwd"),
			runtime.New("tree -L 2 .."),
			// runtime.New("tar -tvf ../duckdb_1.0.0.tar.xz"),
			runtime.New("dput -f -c dput.config duckdb ../duckdb_${DEB_VERSION}_source.changes"),
		)
	}
}

func Build() eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		env.Debug(os.Environ()...)
		return eg.Perform(
			ctx,
			prepare,
			build("jammy"),
		)
	}
}
