package userx

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tracex"
)

const (
	DefaultDir = "eg"
)

const (
	EnvRuntimeDirectory = "RUNTIME_DIRECTORY" // standard environment variable for runtime directories.
	EnvCacheDirectory   = "CACHE_DIRECTORY"   // standard environment variable for cache directories.
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

// DefaultUserConfigLocation looks for rel in the user's config directories. never falls back to the system directory,
// callers create files at the returned path.
func DefaultUserConfigLocation(rel string) string {
	user := CurrentUserOrDefault(Root())
	return DefaultDirectory(
		rel,
		DefaultConfigDirectory(DefaultDir),
		filepath.Join(user.HomeDir, ".config", DefaultDir),
	)
}

// DefaultConfigLocation looks for rel in the user's config directories, then the system directory.
func DefaultConfigLocation(rel string) string {
	user := CurrentUserOrDefault(Root())
	return DefaultDirectory(
		rel,
		DefaultConfigDirectory(DefaultDir),
		filepath.Join(user.HomeDir, ".config", DefaultDir),
		filepath.Join("/etc", DefaultDir),
	)
}

// DefaultConfigDirectory standard config directory for storing data.
func DefaultConfigDirectory(rel ...string) string {
	user := CurrentUserOrDefault(Root())
	// if no directory is specified
	defaultdir := filepath.Join(user.HomeDir, ".config")
	return filepath.Join(envx.String(defaultdir, "CONFIG_DIRECTORY", "XDG_CONFIG_HOME"), filepath.Join(rel...))
}

// DefaultCacheDirectory cache directory for storing data.
func DefaultCacheDirectory(rel ...string) string {
	user := CurrentUserOrDefault(Root())
	if user.Uid == Root().Uid {
		return filepath.Join(envx.String(filepath.Join("/", "var", "cache", DefaultDir), EnvCacheDirectory), filepath.Join(rel...))
	}

	// if no directory is specified
	defaultdir := filepath.Join(user.HomeDir, ".cache", DefaultDir)
	return filepath.Join(envx.String(defaultdir, EnvCacheDirectory, "XDG_CACHE_HOME"), filepath.Join(rel...))
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
		if stringsx.Blank(root) {
			continue
		}

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
