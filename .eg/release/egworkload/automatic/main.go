package main

import (
	"context"
	"log"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
	"github.com/egdaemon/eg/runtime/x/wasi/egpool"
	"github.com/egdaemon/eg/runtime/x/wasi/egx"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile | log.LUTC)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	uploads := egpool.New(4, func(cmd string) eg.OpFn {
		return shell.Op(shell.Env().New(cmd).Attempts(3).Debug())
	})

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		shell.Op(shell.Env().New("eg login")),
		eggithub.DraftClean(0),
		// every workload drafts into the same release, create it once up front so they don't race to create it.
		eggithub.Draft(),
		uploads.Run(egpool.Slice(
			"eg compute upload --no-follow --arch=amd64 --cores=8 --memory=18g release/egworkload/archlinux",
			"eg compute upload --no-follow --arch=amd64 --cores=8 --memory=18g release/egworkload/debian",
		)),
		egx.Retry(0).Delay(30*time.Second).Run(
			eggithub.Promote(
				"egworkload-*.pkg.tar.*",
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
