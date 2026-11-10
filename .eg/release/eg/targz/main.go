package main

import (
	"context"
	"log"
	"path/filepath"

	debeg "eg/compute/debuild/eg"
	egrel "eg/compute/release/eg"

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

	archive := egrel.TarLinux(egarch.Host())
	dir := egtarball.Path(archive)
	runtime := shell.Runtime().
		EnvironFrom(eggolang.Env()...)

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		debeg.Prepare,
		eg.Module(
			ctx,
			debeg.Runner(),
			shell.Op(
				runtime.Newf("rsync -a --mkpath .dist/linux/ %s/", dir),
				// runtime.New("go install ./cmd/eg").Environ("GOBIN", dir+"/usr/bin"),
			),
			eggolang.New().Bin(filepath.Join(dir, "usr", "bin")).Timeout(egenv.TTL()).AutoInstall(),
			shell.Op(
				runtime.Newf("tree -L 3 %s/", dir),
			),
			egtarball.Pack(archive),
			egtarball.SHA256Op(archive),
			eggithub.Draft(egtarball.Archive(archive)),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
