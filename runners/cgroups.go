package runners

// CgroupWorkload is the cgroup, beneath the container's root cgroup, IsolateCgroups moves the workload into.
func CgroupWorkload(runid string) string {
	return "eg.workload." + runid
}
