package computesess

import (
	"context"
	"strconv"
	"time"

	"go.podman.io/podman/v6/pkg/bindings/containers"
)

// ContainerSSHPort the host port podman published for the container's sshd, 0 until the container exists.
// containers are named by the run's id which clients can reuse, so a container left behind by an earlier run
// with the same name still reports its ports until it is replaced. only a container that is running and
// was created since this run started counts.
func ContainerSSHPort(ctx context.Context, cname string, since time.Time) int64 {
	data, err := containers.Inspect(ctx, cname, nil)
	if err != nil || data.NetworkSettings == nil || data.State == nil || !data.State.Running || data.Created.Before(since) {
		return 0
	}

	for _, binding := range data.NetworkSettings.Ports["22/tcp"] {
		if port, err := strconv.ParseInt(binding.HostPort, 10, 64); err == nil && port > 0 {
			return port
		}
	}

	return 0
}

// WaitSSHPort blocks until podman has published a port for the container's sshd and returns it.
// the port is read from the container, which is what makes an automatic port possible: podman picks it.
// returns 0 when ctx is cancelled first.
func WaitSSHPort(ctx context.Context, cname string, since time.Time) int64 {
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return 0
		case <-tick.C:
		}

		if port := ContainerSSHPort(ctx, cname, since); port != 0 {
			return port
		}
	}
}
