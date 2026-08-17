package main

import (
	"context"
	"eg/compute/tarballs"
	"log"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egdmg"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
)

// Baremetal command for darwin due to macosx nonsense for no cloud vms.
func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	runtime := shell.Runtime().
		EnvironFrom(eggolang.Env()...)

	dstdir := filepath.Join(egtarball.Path(tarballs.EgDarwinArm64()), "eg.app", "Contents", "MacOS")
	log.Println("directory", egtarball.Path(tarballs.EgDarwinArm64()))
	log.Println("archive", egtarball.Archive(tarballs.EgDarwinArm64()))
	log.Println("dmg", egdmg.Path(tarballs.EgDarwinArm64()))
	log.Println("release", eggithub.PatternVersion())

	err := eg.Perform(
		ctx,
		eg.Parallel(
			eg.Build(eg.DefaultModule()),
			shell.Op(
				runtime.Newf("go install ./cmd/...").Environ("GOBIN", dstdir),
				runtime.Newf("tree -L 3 %s", egtarball.Path(tarballs.EgDarwinArm64())),
			),
		),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			eggithub.Release(
				egtarball.Archive(tarballs.EgDarwinArm64()),
				egdmg.Path(tarballs.EgDarwinArm64()),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
