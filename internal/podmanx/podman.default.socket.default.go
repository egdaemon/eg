//go:build !darwin

package podmanx

import (
	"fmt"

	"github.com/egdaemon/eg/internal/userx"
)

func DefaultSocket() string {
	return fmt.Sprintf("unix://%s", userx.DefaultRuntimeDirectory("podman", "podman.sock"))
}
