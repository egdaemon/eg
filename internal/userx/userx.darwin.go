//go:build darwin

package userx

import (
	"os/user"
	"path/filepath"

	"github.com/egdaemon/eg/internal/envx"
)

// defaultRuntimeDirectory macOS has no /run. users get a run directory within their cache directory, the same
// layout deeppool uses, which is short enough for the unix sockets bound in it.
func defaultRuntimeDirectory(user *user.User) string {
	if user.Uid == Root().Uid {
		return envx.String(filepath.Join("/", "var", "run"), EnvRuntimeDirectory, "XDG_RUNTIME_DIR")
	}

	return envx.String(filepath.Join(user.HomeDir, ".cache", DefaultDir, "run"), EnvRuntimeDirectory, "XDG_RUNTIME_DIR")
}
