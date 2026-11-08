package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egpool"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile | log.LUTC)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	uploads := egpool.New(4, func(cmd string) eg.OpFn {
		return shell.Op(shell.Env().New(cmd).Attempts(3))
	})

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		shell.Op(shell.Env().New("eg login")),
		// every workload drafts into the same release, create it once up front so they don't race to create it.
		// shell.Op(
		// 	shell.Runtime().
		// 		Environ(_eg.EnvGithubToken, ffigit.Bearer()).
		// 		Newf(
		// 			"gh release view %s || gh release create --draft --target %s %s",
		// 			eggithub.PatternVersion(), eggit.EnvCommit().Hash.String(), eggithub.PatternVersion(),
		// 		),
		// ),
		uploads.Run(egpool.Slice(
			// "eg compute upload --ttl=1h --arch=amd64 --cores=8 --memory=12g release/eg/targz",
			// "eg compute upload --ttl=3h --arch=arm64 --cores=3 --memory=3g release/eg/targz",
			// "eg compute upload --arch=amd64 --cores=8 --memory=6g -e EG_SSH_KEY_SEED=${EG_SSH_KEY_SEED} release/eg/archlinux",
			"eg version",
			"env",
			"eg compute upload --arch=amd64 --cores=8 --memory=6g release/eg/debian",
		)),
		// egx.Retry(0).Delay(30*time.Second).Run(
		// 	eggithub.Promote(
		// 		egtarball.Name(egrel.TarLinux("amd64")),
		// 		egtarball.Name(egrel.TarLinux("arm64")),
		// 		"egd-*.pkg.tar.*",
		// 		"eg_*_amd64.deb",
		// 	),
		// ),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
