package cmdopts

import (
	"path/filepath"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/userx"
)

// DefaultSSHKeyPath returns the path to eg's own managed SSH identity (private
// key; the public key lives alongside it at the same path with a ".pub"
// suffix). This is the key generated/cached by `eg ssh key` and reused by
// every command that authenticates as this machine/account (login,
// registration, actl authorize, compute upload, etc).
func DefaultSSHKeyPath() string {
	user := userx.CurrentUserOrDefault(userx.Root())
	return filepath.Join(
		envx.String(
			filepath.Join(userx.HomeDirectoryOrDefault(user.HomeDir), ".ssh"),
			"CONFIGURATION_DIRECTORY",
		),
		userx.DefaultDir,
	)
}
