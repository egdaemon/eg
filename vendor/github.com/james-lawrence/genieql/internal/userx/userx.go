package userx

import (
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/james-lawrence/genieql/internal/debugx"
	"github.com/james-lawrence/genieql/internal/envx"
)

const (
	DefaultDir = "genieql"
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

// DefaultCacheDirectory cache directory for storing data.
func DefaultCacheDirectory(rel ...string) string {
	defaultdir := userOrRootDir(
		filepath.Join(".cache"),
		filepath.Join("/", "var", "cache"),
	)
	return filepath.Join(envx.String(defaultdir, "CACHE_DIRECTORY", "XDG_CACHE_HOME"), DefaultDir, filepath.Join(rel...))
}

func userOrRootDir(u, root string) string {
	user := CurrentUserOrDefault(Root())
	if user.Uid == Root().Uid {
		return root
	}

	if filepath.IsAbs(u) {
		return u
	}

	return filepath.Join(user.HomeDir, u)
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
