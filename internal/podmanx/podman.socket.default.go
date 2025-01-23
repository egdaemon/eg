//go:build !darwin

package podmanx

import (
	"fmt"
	"path/filepath"

	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/userx"
)

func DefaultSocket() string {
	u := userx.CurrentUserOrDefault(userx.Root())
	socketpath := fsx.LocateFirst(
		filepath.Join("/var", "run", "user", u.Uid, "podman", "podman.sock"),
		filepath.Join("/run", "podman", "podman.sock"),
	)
	return fmt.Sprintf("unix://%s", socketpath)
}
