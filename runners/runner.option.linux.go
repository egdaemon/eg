//go:build linux

package runners

func AgentOptionHostOS() AgentOption {
	return AgentOptionCompose(
		AgentOptionCommandLine("--userns", "host"),       // properly map host user into containers.
		AgentOptionCommandLine("--cap-add", "NET_ADMIN"), // required for loopback device creation inside the container
		AgentOptionCommandLine("--cap-add", "SYS_ADMIN"), // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
		AgentOptionCommandLine("--device", "/dev/fuse"),  // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
		AgentOptionCommandLine("--pids-limit", "-1"),     // ensure we dont run into pid limits.
		AgentOptionCommandLine("--cgroups", "disabled"),
	)
}
