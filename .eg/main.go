package main

import (
	"context"
	"log"

	debian "eg/compute/debuild/eg"

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

	deb := debian.Runner()
	err := eg.Perform(
		ctx,
		// shell.Op(
		// 	shell.New("uname -r"),
		// 	shell.New("cat /proc/cgroups"),
		// 	shell.New("cat /etc/systemd/system.conf").Privileged(),
		// 	shell.New("cat /sys/fs/cgroup/user.slice/user-$(id -u).slice/user@$(id -u).service/cgroup.controllers").Privileged().Lenient(true),
		// 	shell.New("cat /sys/fs/cgroup/user.slice/user-$(id -u).slice/user@$(id -u).service/cgroup.controllers").Lenient(true),
		// 	shell.New("ls -lha /sys/fs/"),
		// ),
		egbug.Fail,
		eggit.AutoClone,
		eg.Parallel(
			eg.Build(eg.DefaultModule()),
			eg.Build(deb.BuildFromFile(".eg/debuild/eg/.debskel/Containerfile")),
		),
		eg.Module(
			ctx,
			deb,
			eg.Sequential(
				eggolang.AutoInstall(),
				eg.Parallel(
					eggolang.AutoTest(),
					IntegrationTests,
				),
				eggolang.RecordCoverage,
			),
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
		eg.Parallel(
			shell.Op(runtime.New("/home/egd/go/bin/eg compute baremetal tests/concurrent")),
			// shell.Op(runtime.New("/home/egd/go/bin/eg compute baremetal tests/containers")),
			// shell.Op(runtime.New("/home/egd/go/bin/eg compute baremetal tests/metrics")),
			// shell.Op(runtime.New("/home/egd/go/bin/eg compute baremetal tests/stress")),
			// shell.Op(runtime.New("/home/egd/go/bin/eg compute baremetal tests/tty")),
			shell.Op(
				runtime.New("/home/egd/go/bin/eg compute baremetal tests/envvars").
					Environ(egbug.EnvUnsafeDigest, "a1cd2e94ecaae6bb5cb1d850484c33db").
					Environ("EG_COMPUTE_MODULE_LEVEL", "0"),
			),
		),
	)
}
