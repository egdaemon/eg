package main

import (
	"context"
	"eg/compute/maintainer"
	egrel "eg/compute/release/eg"
	"log"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egarch"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile | log.LUTC)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()
	pattern := egrel.TarLinux(egarch.Host())
	dir := egtarball.Path(pattern)
	runtime := shell.Env().
		EnvironFrom(eggolang.Env()...)
	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Build(maintainer.Container()),
		eg.Module(
			ctx,
			maintainer.Container(),
			shell.Op(
				runtime.New(egenv.Pattern("echo MODULE; id; ls -ld / /workload /workload/.eg.workspace; env | grep _DIRECTORY | sort; findmnt -T {eg.workload}")).Lenient(true),
				runtime.Newf("echo \"-----------------------------------------------\""),
				runtime.Newf("tree -L 3 %s", egenv.DirectoryWorkload()).Lenient(true),
				runtime.Newf("rsync -a --mkpath .dist/linux/ %s/", dir),
				runtime.Newf("echo \"-----------------------------------------------\""),
				runtime.Newf("tree -L 3 %s", egenv.DirectoryWorkload()).Lenient(true),
				runtime.Newf("echo \"-----------------------------------------------\""),
				runtime.Newf("tree -L 3 %s", dir),
			),
			// same tags as the debian package, except for duckdb_use_lib: the tarball carries only eg, so duckdb is linked in.
			eggolang.New().Bin(egenv.DirectoryWorkload(filepath.Join(dir, "usr", "bin"))).Tags("remote", "exclude_graphdriver_btrfs", "containers_image_openpgp").AutoInstall(),
			eggolang.New().Bin(egenv.DirectoryWorkload("bin")).Tags("remote", "exclude_graphdriver_btrfs", "containers_image_openpgp").AutoInstall(),
			shell.Op(
				runtime.New(egenv.Pattern("echo AFTER; id; ls -ld / /workload /workload/.eg.workspace; env | grep _DIRECTORY | sort; findmnt -T {eg.workload}")).Lenient(true),
				runtime.Newf("echo \"-----------------------------------------------\""),
				runtime.Newf("tree -L 3 %s", egenv.DirectoryWorkload()).Lenient(true),
				runtime.Newf("echo \"-----------------------------------------------\""),
				runtime.Newf("tree -L 3 %s", dir).Lenient(true),
				runtime.Newf("echo \"-----------------------------------------------\""),
				runtime.Newf("tree -L 3 %s", egtarball.Archive(pattern)).Lenient(true),
				runtime.Newf("echo \"-----------------------------------------------\""),
			),
			egtarball.Pack(pattern),
			egtarball.SHA256Op(pattern),
			eg.When(egenv.Draft(), eggithub.Draft(egtarball.Archive(pattern))),
		),
		shell.Op(
			runtime.New(egenv.Pattern("tree -L 3 {eg.workload}")),
			// runtime.New(egenv.Pattern("{eg.workload}/bin/eg --hotswap compute baremetal --ttl=1h --arch=amd64 --cores=8 --memory=12g release/eg/targz")),
			// runtime.New(egenv.Pattern("{eg.workload}/bin/eg --hotswap compute baremetal --no-cgroups --ttl=1h --arch=amd64 --cores=8 --memory=12g release/eg/targz")),
			runtime.New(egenv.Pattern("{eg.workload}/bin/eg --hotswap compute local --ttl=1h --arch=amd64 --cores=8 --memory=12g release/eg/targz")),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
