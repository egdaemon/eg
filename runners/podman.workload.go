package runners

import (
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/podmanx"
)

// PodmanWorkload reports whether this eg runs within a workload container, as the eg that launched the
// container declared, see AgentOptionPodmanWorkload. its podman commands then run with the workload's
// podman configuration, see podmanx.Environ. nothing is detected: a host is never declared one.
func PodmanWorkload() bool {
	return podmanx.Workload()
}

// podmanworkloadlaunch reports whether the workload containers this eg launches are declared to be one.
// a workload declares the workloads it launches, EG_EXPERIMENTAL_PODMAN_WORKLOAD opts a host in.
func podmanworkloadlaunch() bool {
	return PodmanWorkload() || envx.Boolean(false, eg.EnvExperimentalPodmanWorkload)
}

// AgentOptionPodmanWorkload declares the container to be a workload container, see PodmanWorkload.
func AgentOptionPodmanWorkload() AgentOption {
	return envx.Toggle(AgentOptionNoop, AgentOptionEnv(eg.EnvComputePodmanWorkload, "true"), podmanworkloadlaunch())
}

// PodmanWorkloadBuildOptions are the options of the container builds eg runs when PodmanWorkload.
// buildah within a user namespace always connects a build's network namespace with pasta, which then
// cannot join it: setup network: pasta failed: Couldn't open network namespace: Permission denied.
// a build shares the network namespace of the container it runs in instead, within a workload that is
// the workload's own.
func PodmanWorkloadBuildOptions() []string {
	if !PodmanWorkload() {
		return nil
	}

	return []string{"--network", "host"}
}
