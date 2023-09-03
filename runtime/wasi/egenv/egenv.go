package egenv

import (
	"os"
	"path/filepath"
)

func RunID() string {
	return os.Getenv("EG_RUN_ID")
}

func WorkDirectory() string {
	return os.Getenv("EG_ROOT_DIRECTORY")
}

func CacheDirectory() string {
	return os.Getenv("EG_CACHE_DIRECTORY")
}

func CachePath(paths ...string) string {
	return filepath.Join(CacheDirectory(), filepath.Join(paths...))
}
