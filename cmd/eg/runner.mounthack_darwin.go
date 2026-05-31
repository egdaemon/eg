//go:build darwin

package main

import (
	"context"

	"github.com/egdaemon/eg/workspaces"
)

// runContainerMountHack is a no-op on darwin. The macvm guest exposes the
// workspace through virtio-fs with the correct mapped uid already, so the
// Linux-only bindfs permission rewriter has nothing to do.
func runContainerMountHack(_ module, _ context.Context, _ string, _ workspaces.Context) error {
	return nil
}
