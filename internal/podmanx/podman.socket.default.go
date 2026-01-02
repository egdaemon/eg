//go:build !darwin

package podmanx

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/userx"
)

func DefaultSocket() string {
	u := userx.CurrentUserOrDefault(userx.Root())
	upath := filepath.Join("/var", "run", "user", u.Uid, "podman", "podman.sock")
	rpath := filepath.Join("/run", "podman", "podman.sock")
	socketpath := stringsx.DefaultIfBlank(fsx.LocateFirst(upath, rpath), rpath)
	return fmt.Sprintf("unix://%s", socketpath)
}

// EnsureSharedMount is a no-op on Linux where shared mount propagation is typically already configured.
func EnsureSharedMount(ctx context.Context) error {
	return nil
}
