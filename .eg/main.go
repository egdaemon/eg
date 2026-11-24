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

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Parallel(
			eg.Build(eg.DefaultModule()),
		),
		egbug.Log("test suite initiated"),
		eggolang.New().Timeout(10*time.Minute).AutoInstall(),
		// eggolang.New().Timeout(egenv.TTL()).Parallelism(4).Environ("EG_TEST_WASIP1").AutoTest(),
		// egbug.Log("recording coverage initiated"),
		// egcoverage.Report(eggolang.Coverage().MarkGeneratedExt(".gen.go", ".pb.go", ".autogen_test.go")...),
		// copies the binary built by AutoInstall into the workspace directory
		// so the default module's integration tests exercise the new binary.
		shell.Op(shell.Newf("install -m 0755 /home/egd/go/bin/eg %s", sharedbin())),
		egbug.Log("test suite completed"),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			egbug.Log("integration tests initiated"),
			egbug.Debug(
				IntegrationTests,
				egbug.Cgroups,
				egbug.SystemdServices,
				egbug.SystemdFailures,
				cgroupProcesses,
			),
			egbug.Log("integration tests completed"),
		),
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
	runtime := shell.Runtime().
		Privileged().
		Timeout(egenv.TTL())
	// run through the freshly installed binary, it is also what gets mounted into nested modules.
	egbin := sharedbin()
	return eg.Perform(
		ctx,
		shell.Op(
			// envvars is the most likely to break.
			runtime.Newf("%s --hotswap compute local --default-module tests/envvars", egbin),
			runtime.Newf("%s --hotswap compute local --default-module tests/concurrent", egbin),
			runtime.Newf("%s --hotswap compute local --default-module tests/metrics", egbin),
			runtime.Newf("%s --hotswap compute local --default-module tests/stress", egbin),
			runtime.Newf("%s --hotswap compute local --default-module tests/localref", egbin),
			runtime.Newf("%s --hotswap compute local --default-module tests/egdb", egbin),
			// exercises the runner path (spool, queue, archive unpack) which compute local bypasses.
			runtime.Newf("%s --hotswap compute archive --invalidate-cache tests/stress | %s --hotswap compute spooled", egbin, egbin),
			// baremetal inherits the environment of this workload, which carries the container directory layout.
			// regression: chdir /workload/.eg.cache: no such file or directory
			runtime.Newf("%s --hotswap compute baremetal --no-podman tests/baremetal", egbin).Lenient(true),
			runtime.Newf("%s --hotswap compute baremetal tests/baremetal-virtualization", egbin).Lenient(true),
			// runtime.Newf("%s --hotswap compute baremetal -vvvv tests/tty", egbin),
			// runtime.Newf("%s --hotswap compute baremetal tests/containers", egbin),
			// runtime.Newf("%s --hotswap compute baremetal -vv tests/gpgagent", egbin),
		),
	)
}

// temporary debugging: where do the processes live relative to the container's cgroup tree.
func cgroupProcesses(ctx context.Context, op eg.Op) error {
	lenient := shell.Runtime().Lenient(true)
	privileged := lenient.Privileged()
	return shell.Run(
		ctx,
		lenient.New("echo 'system.slice children:' && ls -lhan /sys/fs/cgroup/system.slice | grep '^d'"),
		lenient.New("echo 'process tree with cgroups:' && ps -eo pid,ppid,cgroup:60,args --forest | head -n 80"),
		privileged.New("echo 'podman.service cgroup:' && ls -lhan /sys/fs/cgroup/system.slice/podman.service && cat /sys/fs/cgroup/system.slice/podman.service/cgroup.procs"),
		lenient.New("echo 'cgroup.procs of root and init.scope:' && cat /sys/fs/cgroup/init.scope/cgroup.procs"),
		lenient.New("echo 'versions:' && crun --version | head -n 1 && podman --version && conmon --version | head -n 1"),
		lenient.New("echo 'root cgroup procs (host pids):' && cat /sys/fs/cgroup/cgroup.procs && echo 'cgroup.type:' && cat /sys/fs/cgroup/cgroup.type"),
		lenient.New("echo 'pid 2 cgroup:' && cat /proc/2/cgroup && grep -E '^(Name|PPid|NSpid)' /proc/2/status"),
		privileged.New("echo 'module binary:' && for p in $(pgrep -f 'eg module'); do echo pid $p; readlink /proc/$p/exe; tr '\\0' '\\n' < /proc/$p/environ | grep -E 'NESTED_LEVEL|EG_COMPUTE_BIN|EG_COMPUTE_RUN_ID'; done; echo 'binary has isolation marker:'; grep -c 'isolating cgroups initiated' /eg.mnt/.eg.runtime/egbin $(which eg) /home/egd/go/bin/eg /usr/bin/eg 2>&1; echo 'eg versions:'; /eg.mnt/.eg.runtime/egbin version 2>&1 | head -2; eg version 2>&1 | head -2"),
		lenient.New("echo 'self status:' && grep -E '^(NSpid|PPid)' /proc/self/status"),
		lenient.New("echo 'containers.conf:' && cat /etc/containers/containers.conf /usr/share/containers/containers.conf 2>&1 | grep -E 'cgroup|runtime|^\\['"),
	)
}
