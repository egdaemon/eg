package userx

import (
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/james-lawrence/eg/internal/envx"
)

const (
	DefaultDir = "eg"
)

func fallbackUser() user.User {
	return user.User{
		Gid:     "0",
		Uid:     "0",
		HomeDir: "/root",
	}
}

// CurrentUserOrDefault returns the current user or the default configured user.
// (usually root)
func CurrentUserOrDefault(d user.User) (result *user.User) {
	var (
		err error
	)

	if result, err = user.Current(); err != nil {
		log.Println("failed to retrieve current user, using default", err)
		tmp := d
		return &tmp
	}

	return result
}

// DefaultUserDirLocation returns the user directory location.
func DefaultUserDirLocation(name string) string {
	user := CurrentUserOrDefault(fallbackUser())

	envconfig := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), DefaultDir)
	home := filepath.Join(user.HomeDir, ".config", DefaultDir)

	return DefaultDirectory(name, envconfig, home)
}

// DefaultDirLocation looks for a directory one of the default directory locations.
func DefaultDirLocation(rel string) string {
	user := CurrentUserOrDefault(fallbackUser())

	env := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), DefaultDir)
	home := filepath.Join(user.HomeDir, ".config", DefaultDir)
	system := filepath.Join("/etc", DefaultDir)

	return DefaultDirectory(rel, env, home, system)
}

// DefaultCacheDirectory cache directory for storing data.
func DefaultCacheDirectory() string {
	user := CurrentUserOrDefault(fallbackUser())
	if user.Uid == fallbackUser().Uid {
		return filepath.Join("/", "var", "cache", DefaultDir)
	}

	root := envx.String(filepath.Join(user.HomeDir, ".cache"), "CACHE_DIRECTORY", "XDG_CACHE_HOME")

	return filepath.Join(root, DefaultDir)
}

// DefaultDirectory finds the first directory root that exists and then returns
// that root directory joined with the relative path provided.
func DefaultDirectory(rel string, roots ...string) (path string) {
	for _, root := range roots {
		path = filepath.Join(root, rel)
		if _, err := os.Stat(root); err == nil {
			return path
		}
	}

	return path
}
