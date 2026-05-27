//go:build !darwin

package main

import (
	"context"

	"github.com/egdaemon/eg/workspaces"
)

// runContainerMountHack delegates to the bindfs-based permission remap on
// platforms where the eg container runtime depends on FUSE. The macvm
// guest has no podman / no bindfs, so the darwin build emits a no-op.
func runContainerMountHack(t module, ctx context.Context, runid string, ws workspaces.Context) error {
	return t.mounthack(ctx, runid, ws)
}
