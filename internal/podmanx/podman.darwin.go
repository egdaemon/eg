//go:build darwin

package podmanx

import (
	"fmt"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/userx"
)

func DefaultSocket() string {
	socket := errorsx.Zero(userx.HomeDirectory(".local", "share", "containers", "podman", "machine", "podman.sock"))
	return fmt.Sprintf("unix://%s", socket)
}
