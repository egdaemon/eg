//go:build darwin

package podmanx

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/userx"
)

func DefaultSocket() string {
	socket := errorsx.Zero(userx.HomeDirectory(".local", "share", "containers", "podman", "machine", "podman.sock"))
	return fmt.Sprintf("unix://%s", socket)
}

// EnsureSharedMount ensures the Podman Machine VM has shared mount propagation on root.
// This is required for rslave mount propagation to work in nested containers.
func EnsureSharedMount(ctx context.Context) error {
	log.Println("ensuring podman machine has shared root mount propagation")
	cmd := exec.CommandContext(ctx, "podman", "machine", "ssh", "--", "sudo", "mount", "--make-rshared", "/")
	if output, err := cmd.CombinedOutput(); err != nil {
		return errorsx.Wrapf(err, "failed to set shared mount propagation: %s", string(output))
	}
	return nil
}
