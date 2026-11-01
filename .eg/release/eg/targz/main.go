package main

import (
	"context"
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
	runtime := shell.Runtime().
		EnvironFrom(eggolang.Env()...)

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		shell.Op(
			runtime.Newf("rsync -a --mkpath .dist/linux/ %s/", dir),
		),
		eggolang.New().Bin(egenv.DirectoryWorkload(filepath.Join(dir, "usr", "bin"))).Tags("remote", "exclude_graphdriver_btrfs", "containers_image_openpgp").AutoInstall(),
		egtarball.Pack(pattern),
		egtarball.SHA256Op(pattern),
		eg.When(egenv.Draft(), eggithub.Draft(egtarball.Archive(pattern))),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
