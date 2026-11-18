package runners

import (
	"strings"

	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
)

// Nested reports whether the process is running inside a podman container, or inside a Linux machine,
// whose systemd records the machine as its container manager. Neither lets podman configure networks.
func Nested() bool {
	return envx.String("", "container") == "podman" || strings.TrimSpace(errorsx.ZeroSilent(fsx.String("/run/systemd/container"))) == egvm.Container
}
