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
	upath := filepath.Join("/var", "run", "user", u.Uid, "podman", "podman.sock")
	socketpath := fsx.LocateFirst(
		upath,
		filepath.Join("/run", "podman", "podman.sock"),
	)
	return fmt.Sprintf("unix://%s", socketpath)
}
