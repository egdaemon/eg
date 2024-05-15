package egenv

import (
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/env"
)

// Provides the TTL specified by the runtime. used for setting context durations.
// defaults to an hour. currently not fully implemented.
func TTL() time.Duration {
	return env.Duration(time.Hour, "EG_TTL")
}

func RunID() string {
	return os.Getenv("EG_RUN_ID")
}

func CacheDirectory(paths ...string) string {
	return filepath.Join(env.String(os.TempDir(), "EG_CACHE_DIRECTORY", "CACHE_DIRECTORY"), filepath.Join(paths...))
}

func RuntimeDirectory(paths ...string) string {
	return filepath.Join(env.String(os.TempDir(), "EG_RUNTIME_DIRECTORY"), filepath.Join(paths...))
}
