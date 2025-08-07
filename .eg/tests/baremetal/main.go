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
			// shell.New("systemctl status podman.socket"),
			shell.Newf("mkdir -p %s", filepath.Join(egtarball.Path("example"), "Contents")),
			shell.Newf("echo \"derp\" | tee %s/hello.world.txt", egtarball.Path("example")),
			shell.Newf("echo \"derp\" | tee %s/Info.plist", filepath.Join(egtarball.Path("example"), "Contents")),
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
				shell.Newf("tar tf %s | md5sum", egtarball.Archive("example")),
				shell.Newf("test \"$(tar tf %s | md5sum)\" = \"c22870ddb29fce64f00a4c3570644acb  -\"", egtarball.Archive("example")),
			),
			egbug.Log("tar archive is missing contents"),
		),
	)(ctx, op)
}

func TestModule(ctx context.Context, op eg.Op) error {
	b := egdmg.New("retrovibe", egdmg.OptionBuildDir(egenv.CacheDirectory(".dist", "retrovibed.darwin.arm64")))
	return eg.Perform(
		ctx,
		egbug.DirectoryTree(egtarball.Path("example")),
		egdmg.Build(b, os.DirFS(egtarball.Path("example"))),
	)
}

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		egbug.Log("init"),
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
			egbug.EnsureEnv("26941e25a2adf90a2298f05ded6f1243", egbug.EgEnviron()...),
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
					egbug.EnsureEnv("15dbf130e4ed546fa5b9e8799b170cdc", egbug.EgEnviron()...),
					egbug.Log("container module environment has drifted"),
				),
				TestModule,
			),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
