package main

import (
	"context"
	egrel "eg/compute/release/eg"
	"log"

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
		eg.Build(eg.DefaultModule()),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			shell.Op(
				runtime.Newf("rsync -a --mkpath .dist/linux/ %s/", dir),
				runtime.New("go install ./cmd/eg").Environ("GOBIN", dir+"/usr/bin"),
				// egtarball.Pack produces .tar.xz, we publish .tar.gz.
				runtime.Newf("tar -C %s -zcvf %s .", dir, egtarball.Archive(archive)),
			),
			egtarball.SHA256Op(archive),
			eggithub.Draft(egtarball.Archive(archive)),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
