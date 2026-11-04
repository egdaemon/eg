//go:build darwin

package userx

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/egdaemon/eg/internal/envx"
)

// defaultRuntimeDirectory macOS has no /run. users get the private temporary directory launchd creates for them
// under /var/folders, which is cleared on reboot like /run/user and short enough for the unix sockets bound in it.
func defaultRuntimeDirectory(user *user.User) string {
	if user.Uid == Root().Uid {
		return envx.String(filepath.Join("/", "var", "run"), EnvRuntimeDirectory, "XDG_RUNTIME_DIR")
	}

	return envx.String(os.TempDir(), EnvRuntimeDirectory, "XDG_RUNTIME_DIR")
}
