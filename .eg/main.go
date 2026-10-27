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
		eg.Module(
			ctx,
			c1,
			eggolang.New().Timeout(10*time.Minute).AutoInstall(),
			eggolang.New().Timeout(10*time.Minute).AutoTest(),
			egbug.Log("recording coverage initiated"),
			eggolang.RecordCoverage,
		),
		eg.Module(
			ctx,
			// eg default module should generally be stable
			// allowing these integration tests to be consistent
			// w/ respect to behavior particularly regarding envvars.
			eg.DefaultModule(),
			egbug.Log("integration tests initiated"),
			IntegrationTests,
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}

func IntegrationTests(ctx context.Context, op eg.Op) error {
	// privileged is required to access the podman socket.
	// which works fine for the standard container runner
	// because the process is initiated as root.
	// but inside of a workload we need to reup privileges.
	runtime := shell.Runtime().Privileged()
	return eg.Perform(
		ctx,
		shell.Parallel(
			runtime.New("/home/egd/go/bin/eg compute baremetal tests/concurrent"),
			runtime.New("/home/egd/go/bin/eg compute baremetal tests/metrics"),
			runtime.New("/home/egd/go/bin/eg compute baremetal tests/stress"),
			runtime.New("/home/egd/go/bin/eg compute baremetal tests/localref"),
			runtime.New("/home/egd/go/bin/eg compute baremetal tests/egdb"),
			runtime.New("/home/egd/go/bin/eg compute baremetal tests/envvars").
				Environ(egenv.EnvComputeModuleNestedLevel, "0"),
			// runtime.New("/home/egd/go/bin/eg compute baremetal -vvvv tests/tty"),
			// runtime.New("/home/egd/go/bin/eg compute baremetal tests/containers"),
			// runtime.New("/home/egd/go/bin/eg compute baremetal -vv tests/gpgagent"),
		),
	)
}
