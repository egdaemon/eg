package runners

import (
	"log"

	"github.com/egdaemon/eg/internal/fsx"
)

// AgentOptionSSHKnownHosts mounts the host's known hosts file read only as the
// container's system wide known hosts. when the file does not exist the container's
// own /etc/ssh/ssh_known_hosts is left in place, otherwise podman refuses to create
// the container and every workload fails.
func AgentOptionSSHKnownHosts(path string) AgentOption {
	if !fsx.FileExists(path) {
		log.Println("ssh known hosts file missing, workloads will use the container's known hosts", path)
		return AgentOptionNoop
	}

	return AgentOptionVolumes(AgentMountReadOnly(path, "/etc/ssh/ssh_known_hosts"))
}
