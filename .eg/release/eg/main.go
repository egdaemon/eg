package main

import (
	"context"
	"log"

	"eg/compute/archlinux"
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
			archlinux.Prepare,
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
			// eg.Module(
			// 	ctx,
			// 	archlinux.AURRunner(),
			// 	eg.Sequential(
			// 		archlinux.Publish,
			// 	),
			// ),
		),
		eggithub.Draft(
			egenv.CacheDirectory(".dist", "*.deb"),
		),
		eggithub.Promote(
			"eg_*_amd64.deb",
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
