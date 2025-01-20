//go:build !darwin

package userx

import (
	"os/user"
	"path/filepath"

	"github.com/egdaemon/eg/internal/envx"
)

func defaultRuntimeDirectory(user *user.User) string {
	if user.Uid == Root().Uid {
		return envx.String(filepath.Join("/", "run"), "RUNTIME_DIRECTORY", "XDG_RUNTIME_DIR")
	}

	defaultdir := filepath.Join("/", "run", "user", user.Uid)
	return envx.String(defaultdir, "RUNTIME_DIRECTORY", "XDG_RUNTIME_DIR")
}
