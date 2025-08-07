package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
	"github.com/egdaemon/eg/runtime/x/wasi/egdmg"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
)

const (
	tarball = "macos.dmg"
)

// test archive simulating a macosx dmg tree.
func Archive(ctx context.Context, op eg.Op) error {
	return eg.Sequential(
		shell.Op(
			shell.Newf("ls -lha %s", egenv.WorkloadDirectory()),
			shell.Newf("mkdir -p %s", filepath.Join(egtarball.Path(tarball), "Contents")),
			shell.Newf("echo \"derp\" | tee %s/Info.plist", filepath.Join(egtarball.Path(tarball), "Contents")),
		),

		egtarball.Pack(tarball),
		egtarball.SHA256Op(tarball),
	)(ctx, op)
}

// create the dmg from within a module
func Dmg(ctx context.Context, op eg.Op) error {
	b := egdmg.New("retrovibe", egdmg.OptionBuildDir(egenv.CacheDirectory(".dist", "retrovibed.darwin.arm64")))
	return eg.Perform(
		ctx,
		egbug.DirectoryTree(egtarball.Path(tarball)),
		egdmg.Build(b, os.DirFS(egtarball.Path(tarball))),
	)
}

// Baremetal command for darwin due to macosx nonsense for no cloud vms.
func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Parallel(
			eg.Build(eg.DefaultModule()),
			Archive,
		),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			Dmg,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
