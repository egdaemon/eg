package egenv

import (
	"os"
	"path/filepath"
)

func RunID() string {
	return os.Getenv("EG_RUN_ID")
}

func GitCommit() string {
	return os.Getenv("EG_GIT_COMMIT")
}

func CacheDirectory() string {
	return os.Getenv("EG_CACHE_DIRECTORY")
}

func CachePath(paths ...string) string {
	return filepath.Join(CacheDirectory(), filepath.Join(paths...))
}

func RuntimeDirectory() string {
	return os.Getenv("EG_RUNTIME_DIRECTORY")
}
