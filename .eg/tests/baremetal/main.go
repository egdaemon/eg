package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
)

func Debug(runtime shell.Command) eg.OpFn {
	return eg.Sequential(
		egbug.Log("---------------------------- failed inspection initiated ----------------------------"),
		shell.Op(
			runtime.New("env | sort"),
			runtime.New("id"),
		),
		egbug.Log("---------------------------- failed inspection completed ----------------------------"),
	)
}

// test case for baremetal. minimic the original use case of darwin builds.
func Test(ctx context.Context, op eg.Op) error {
	return eg.Sequential(
		shell.Op(
			shell.Newf("mkdir -p %s", egtarball.Path("example")),
			shell.Newf("echo \"derp\" | tee %s/hello.world.txt", egtarball.Path("example")),
		),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			eg.Sequential(
				egtarball.Pack("example"),
				egtarball.SHA256Op("example"),
			),
		),
		egbug.DebugFailure(
			// ensure the tar archive has the expected files.
			shell.Op(
				shell.Newf("test \"$(tar tf %s | md5sum)\" = \"c900687098f86ddff70bd4e7abb9bf29  -\"", egtarball.Archive("example")),
			),
			egbug.Log("tar archive is missing contents"),
		),
	)(ctx, op)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Build(eg.DefaultModule()),
		egbug.Log("baremetal tarball"),
		Test,
		egbug.Log("baremetal basic sanity checks"),
		egbug.DebugFailure(
			// ensure that the user isnt egd
			shell.Op(
				shell.New("test $(id -nu) != egd"),
				shell.New("test $(id -ng) != egd"),
			),
			Debug(shell.Runtime()),
		),
		// test for cache directory and runtime.
		// test for git commit details.
		egbug.DebugFailure(
			egbug.EnsureEnv("307c7421f028bb223c6dd928a1a5b328", egbug.EgEnviron()...),
			egbug.Log("baremetal environment has drifted"),
		),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			eg.Sequential(
				egbug.DebugFailure(
					// ensure that the user isnt egd
					shell.Op(
						shell.New("test $(id -nu) = egd"),
						shell.New("test $(id -ng) = egd"),
					),
					Debug(shell.Runtime()),
				),
				egbug.DebugFailure(
					egbug.EnsureEnv("514d01ba58d6836f55e1efdcb76ee548", egbug.EgEnviron()...),
					egbug.Log("container module environment has drifted"),
				),
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
