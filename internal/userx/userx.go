package userx

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
)

const (
	DefaultDir = "eg"
)

func Root() user.User {
	return user.User{
		Gid:     "0",
		Uid:     "0",
		HomeDir: "/root",
	}
}

// CurrentUserOrDefault returns the current user or the default configured user.
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
	user := CurrentUserOrDefault(Root())

	envconfig := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), DefaultDir)
	home := filepath.Join(user.HomeDir, ".config", DefaultDir)

	return DefaultDirectory(name, envconfig, home)
}

// DefaultDirLocation looks for a directory one of the default directory locations.
func DefaultDirLocation(rel string) string {
	user := CurrentUserOrDefault(Root())

	env := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), DefaultDir)
	home := filepath.Join(user.HomeDir, ".config", DefaultDir)
	system := filepath.Join("/etc", DefaultDir)

	return DefaultDirectory(rel, env, home, system)
}

// DefaultCacheDirectory cache directory for storing data.
func DefaultCacheDirectory() string {
	user := CurrentUserOrDefault(Root())
	if user.Uid == Root().Uid {
		return envx.String(filepath.Join("/", "var", "cache", DefaultDir), "CACHE_DIRECTORY")
	}

	root := filepath.Join(user.HomeDir, ".cache", DefaultDir)

	return envx.String(root, "CACHE_DIRECTORY", "XDG_CACHE_HOME")
}

// DefaultRuntimeDirectory runtime directory for storing data.
func DefaultRuntimeDirectory() string {
	user := CurrentUserOrDefault(Root())

	if user.Uid == Root().Uid {
		return envx.String(filepath.Join("/", "run", DefaultDir), "RUNTIME_DIRECTORY", "XDG_RUNTIME_DIR")
	}

	defaultdir := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s", DefaultDir, "runtime"))

	return envx.String(defaultdir, "RUNTIME_DIRECTORY", "XDG_RUNTIME_DIR")
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

// HomeDirectoryOrDefault loads the user home directory or fallsback to the provided
// path when an error occurs.
func HomeDirectoryOrDefault(fallback string) (dir string) {
	var (
		err error
	)

	if dir, err = os.UserHomeDir(); err != nil {
		debugx.Println("unable to get user home directory", err)
		return fallback
	}

	return dir
}
