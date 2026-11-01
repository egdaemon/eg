// Package userx mirrors the small slice of retroapi/userx this module needs
// (the XDG runtime-directory resolution), kept dependency-free so konggdx
// stays standalone.
package userx

import (
	"log"
	"os/user"
	"path/filepath"

	"github.com/egdaemon/gdx/internal/envx"
)

func root() user.User {
	return user.User{
		Gid:     "0",
		Uid:     "0",
		HomeDir: "/root",
	}
}

// currentUserOrDefault returns the current user or the default configured user.
func currentUserOrDefault(d user.User) (result *user.User) {
	var err error

	if result, err = user.Current(); err != nil {
		log.Println("failed to retrieve current user, using default", err)
		tmp := d
		return &tmp
	}

	return result
}

// RuntimeDirectory returns a path within the user's XDG runtime directory
// ($RUNTIME_DIRECTORY or $XDG_RUNTIME_DIR, falling back to /run for root or
// /run/user/<uid> otherwise).
func RuntimeDirectory(rel ...string) string {
	u := currentUserOrDefault(root())

	if u.Uid == root().Uid {
		return envx.String(filepath.Join("/", "run"), "RUNTIME_DIRECTORY", "XDG_RUNTIME_DIR")
	}

	defaultdir := filepath.Join("/", "run", "user", u.Uid)
	return filepath.Join(envx.String(defaultdir, "RUNTIME_DIRECTORY", "XDG_RUNTIME_DIR"), filepath.Join(rel...))
}
