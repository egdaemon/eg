package main

import (
	"context"
	"log"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
	"github.com/egdaemon/eg/runtime/x/wasi/egdb"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	c1 := eg.Container("eg.ubuntu")
	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Parallel(
			eg.Build(eg.DefaultModule()),
			eg.Build(c1.BuildFromFile(".eg/Containerfile")),
		),
		egbug.Log("test suite initiated"),
		eg.Module(
			ctx,
			c1,
			eggolang.New().Timeout(10*time.Minute).AutoInstall(),
			eggolang.New().Timeout(10*time.Minute).AutoTest(),
			egbug.Log("recording coverage initiated"),
			eggolang.RecordCoverage,
			// copies the binary built by AutoInstall into the workspace directory
			// so the default module's integration tests exercise the new binary.
			// shell.Op(shell.Newf("install -m 0755 /home/egd/go/bin/eg %s", sharedbin()).Debug()),
		),
		egbug.Log("test suite completed"),
		// egbug.Log("integration tests initiated"),
		// IntegrationTests,
		// egbug.Log("integration tests completed"),
		// keep the analytics database of a successful run in the cache for the next run, see egdb.Latest.
		egdb.Cache,
	)

	if err != nil {
		log.Fatalln(err)
	}
}

// path of the freshly installed eg binary within the workspace directory.
// the workspace is shared with nested modules, the ephemeral directory is not.
func sharedbin() string {
	return egenv.WorkspaceDirectory("eg")
}

func IntegrationTests(ctx context.Context, op eg.Op) error {
	// privileged is required to access the podman socket.
	// which works fine for the standard container runner
	// because the process is initiated as root.
	// but inside of a workload we need to reup privileges.
	runtime := shell.Runtime().Privileged().Debug()
	// run through the freshly installed binary, it is also what gets mounted into nested modules.
	egbin := sharedbin()
	return eg.Perform(
		ctx,
		shell.Op(
			runtime.Newf("%s --hotswap compute baremetal --no-cgroups tests/concurrent", egbin),
			runtime.Newf("%s --hotswap compute baremetal --no-cgroups tests/metrics", egbin),
			runtime.Newf("%s --hotswap compute baremetal --no-cgroups tests/stress", egbin),
			runtime.Newf("%s --hotswap compute baremetal --no-cgroups tests/localref", egbin),
			runtime.Newf("%s --hotswap compute baremetal --no-cgroups tests/egdb", egbin),
			runtime.Newf("%s --hotswap compute baremetal --no-cgroups tests/envvars", egbin),
			// runtime.New("/home/egd/go/bin/eg compute baremetal -vvvv tests/tty"),
			// runtime.New("/home/egd/go/bin/eg compute baremetal tests/containers"),
			// runtime.New("/home/egd/go/bin/eg compute baremetal -vv tests/gpgagent"),
		),
	)
}
