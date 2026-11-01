package main

import (
	"context"
	egrel "eg/compute/release/eg"
	"log"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
	"github.com/egdaemon/eg/runtime/x/wasi/egpool"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
	"github.com/egdaemon/eg/runtime/x/wasi/egx"

	_eg "github.com/egdaemon/eg"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile | log.LUTC)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	uploads := egpool.New(2, func(cmd string) eg.OpFn {
		return shell.Op(shell.Env().New(cmd).Attempts(3))
	})

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		shell.Op(
			shell.Env().New("env"),
			shell.Env().New("eg register --account egd"),
			shell.Env().New("eg login"),
		),
		// every workload drafts into the same release, create it once up front so they don't race to create it.
		eg.When(egenv.Draft(), shell.Op(
			shell.Runtime().
				Environ(_eg.EnvGithubToken, ffigit.Bearer()).
				Newf(
					"gh release view %s || gh release create --draft --target %s %s",
					eggithub.PatternVersion(), eggit.EnvCommit().Hash.String(), eggithub.PatternVersion(),
				),
		)),
		uploads.Run(egpool.Slice(
			"eg compute upload --ttl=1h --arch=amd64 --cores=8 --memory=12g release/eg/targz",
			// "eg compute upload --ttl=3h --arch=arm64 --cores=3 --memory=3g release/eg/targz",
			// "eg compute upload --arch=amd64 --cores=8 --memory=6g release/eg/archlinux",
			// "eg compute upload --arch=amd64 --cores=8 --memory=6g release/eg/debian",
		)),
		eg.When(egenv.Release(), egx.Retry(0).Delay(30*time.Second).Run(
			eggithub.Promote(
				egtarball.Name(egrel.TarLinux("amd64")),
				egtarball.Name(egrel.TarLinux("arm64")),
				"egd-*.pkg.tar.*",
				"eg_*_amd64.deb",
			),
		)),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
