package runners

import "github.com/egdaemon/eg/internal/envx"

// Nested reports whether the process is running inside a podman container.
func Nested() bool {
	return envx.String("", "container") == "podman"
}
