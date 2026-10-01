package userx

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/tracex"
)

const (
	DefaultDir = "eg"
)

func IsRoot(u *user.User) bool {
	return u.Uid == Root().Uid
}

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
		tracex.Println("failed to retrieve current user, using default", err)
		tmp := d
		return &tmp
	}

	return result
}

// CurrentUsername returns the username of the user running this process,
// falling back to d when it can't be determined.
func CurrentUsername(d string) string {
	u, err := user.Current()
	if err != nil {
		tracex.Println("failed to retrieve current user, using default", err)
		return d
	}

	return u.Username
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

// ConfigDirectory standard config directory for storing data.
func ConfigDirectory(rel ...string) string {
	user := CurrentUserOrDefault(Root())
	// if no directory is specified
	defaultdir := filepath.Join(user.HomeDir, ".config")
	return filepath.Join(envx.String(defaultdir, "CONFIG_DIRECTORY", "XDG_CONFIG_HOME"), filepath.Join(rel...))
}

// DefaultCacheDirectory cache directory for storing data.
func DefaultCacheDirectory(rel ...string) string {
	user := CurrentUserOrDefault(Root())
	if user.Uid == Root().Uid {
		return filepath.Join(envx.String(filepath.Join("/", "var", "cache", DefaultDir), "CACHE_DIRECTORY"), filepath.Join(rel...))
	}

	// if no directory is specified
	defaultdir := filepath.Join(user.HomeDir, ".cache", DefaultDir)
	return filepath.Join(envx.String(defaultdir, "CACHE_DIRECTORY", "XDG_CACHE_HOME"), filepath.Join(rel...))
}

// DefaultRuntimeDirectory runtime directory for storing data.
func DefaultRuntimeDirectory(rel ...string) string {
	user := CurrentUserOrDefault(Root())
	return filepath.Join(defaultRuntimeDirectory(user), filepath.Join(rel...))
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

// HomeDirectory loads the user home directory.
func HomeDirectory(rel ...string) (dir string, err error) {
	if dir, err = os.UserHomeDir(); err != nil {
		return "", err
	}

	return filepath.Join(dir, filepath.Join(rel...)), nil
}

// DefaultSSHKeyPath returns the path to eg's own managed SSH identity (private
// key; the public key lives alongside it at the same path with a ".pub"
// suffix). This is the key generated/cached by `eg ssh key` and reused by
// every command that authenticates as this machine/account (login,
// registration, actl authorize, compute upload, etc).
func DefaultSSHKeyPath() string {
	user := CurrentUserOrDefault(Root())
	return filepath.Join(
		envx.String(
			filepath.Join(HomeDirectoryOrDefault(user.HomeDir), ".ssh"),
			"CONFIGURATION_DIRECTORY",
		),
		DefaultDir,
	)
}
