package runners

import (
	"os"
	"strings"

	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
)

// Nested reports whether the process is running inside a podman container, or inside a Linux machine,
// whose systemd records the machine as its container manager. Neither lets podman configure networks.
func Nested() bool {
	return envx.String("", "container") == "podman" || strings.TrimSpace(string(errorsx.Zero(os.ReadFile("/run/systemd/container")))) == egvm.Container
}
