package main

import (
	"context"
	"log"

	debeg "eg/compute/debuild/eg"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
)

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Parallel(
			debeg.Prepare,
		),
		eg.Parallel(
			eg.Module(
				ctx,
				debeg.Runner(),
				eg.Sequential(
					shell.Op(
						// clean up old debians. remove in future version.
						shell.Newf("rm %s", egenv.CacheDirectory(".dist", "*.deb")).Lenient(true),
					),
					debeg.Build,
					debeg.Upload,
					shell.Op(
						shell.Newf("cp %s/*.deb %s", egenv.EphemeralDirectory("deb.eg"), egenv.CacheDirectory(".dist")),
					),
				),
			),
		),
		func(ctx context.Context, o eg.Op) error {
			return eggithub.Release(
				egenv.CacheDirectory(".dist", "*.deb"),
			)(ctx, o)
		},
	)

	if err != nil {
		log.Fatalln(err)
	}
}
