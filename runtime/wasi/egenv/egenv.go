package egenv

import (
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/runtime/wasi/env"
)

func RunID() string {
	return os.Getenv("EG_RUN_ID")
}

func GitCommit() string {
	return os.Getenv("EG_GIT_COMMIT")
}

func CacheDirectory(paths ...string) string {
	return filepath.Join(env.String(os.TempDir(), "EG_CACHE_DIRECTORY"), filepath.Join(paths...))
}

func RuntimeDirectory() string {
	return os.Getenv("EG_RUNTIME_DIRECTORY")
}
