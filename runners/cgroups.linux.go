//go:build linux

package runners

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
	"go.podman.io/common/pkg/cgroups"
)

// IsolateCgroups moves the workload out of the container's root cgroup into eg.workload.{runid}, and hands
// the root's controllers to its children, so the container's systemd, and containers run by the workload, get
// the controllers they would on the host. podman exec places the workload in the container's root cgroup,
// and a cgroup holding processes can not hand its children controllers: nested containers fail with
// `controller pids is not available`. the workload's cgroup repeats the container's limits, its commands
// read the limits of their own cgroup. see the machine's delegate in internal/egvm.
//
// does nothing when the process is not in the root of a cgroup namespace, i.e.) it runs on the host.
func IsolateCgroups(runid string) (err error) {
	var (
		current     string
		controllers string
		limit       string
		root        = "/sys/fs/cgroup"
		workload    = filepath.Join(root, CgroupWorkload(runid))
	)

	if current, err = cgroups.GetOwnCgroup(); err != nil {
		return errorsx.Wrap(err, "unable to determine the workload's cgroup")
	}

	if current != "/" || os.Getpid() == 1 {
		return nil
	}

	if err = cgroups.MoveUnderCgroupSubtree(CgroupWorkload(runid)); err != nil {
		return errorsx.Wrap(err, "unable to move the workload out of the root cgroup")
	}

	if controllers, err = cgroups.ReadFile(root, "cgroup.controllers"); err != nil {
		return errorsx.Wrap(err, "unable to read the root cgroup's controllers")
	}

	for _, c := range strings.Fields(controllers) {
		if err = cgroups.WriteFile(root, "cgroup.subtree_control", "+"+c); err != nil {
			return errorsx.Wrapf(err, "unable to delegate the %s controller", c)
		}
	}

	for _, file := range []string{"cpu.max", "memory.max"} {
		if limit, err = cgroups.ReadFile(root, file); err != nil {
			return errorsx.Wrapf(err, "unable to read %s", file)
		}

		if err = cgroups.WriteFile(workload, file, strings.TrimSpace(limit)); err != nil {
			return errorsx.Wrapf(err, "unable to write %s", file)
		}
	}

	return nil
}
