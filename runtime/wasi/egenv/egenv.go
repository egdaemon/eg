package egenv

import (
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/env"
)

// Provides the TTL specified by the runtime. used for setting context durations.
// defaults to an hour. currently not fully implemented.
func TTL() time.Duration {
	return env.Duration(time.Hour, eg.EnvComputeTTL)
}

func RunID() string {
	return os.Getenv(eg.EnvComputeRunID)
}

func CacheDirectory(paths ...string) string {
	return filepath.Join(env.String(os.TempDir(), eg.EnvComputeCacheDirectory, "CACHE_DIRECTORY"), filepath.Join(paths...))
}

func RuntimeDirectory(paths ...string) string {
	return filepath.Join(env.String(os.TempDir(), eg.EnvComputeRuntimeDirectory), filepath.Join(paths...))
}

func EphemeralDirectory(paths ...string) string {
	return filepath.Join(os.TempDir(), filepath.Join(paths...))
}

func RootDirectory(paths ...string) string {
	return filepath.Join(env.String(eg.DefaultRootDirectory, eg.EnvComputeRootDirectory), filepath.Join(paths...))
}
