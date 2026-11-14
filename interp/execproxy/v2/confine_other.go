//go:build !linux

package execproxy

import (
	"context"
	"os/exec"
)

// confined is the process that runs name with args; only Linux confines it.
func confined(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
