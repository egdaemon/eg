package egenv

import (
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/runtime/wasi/env"
)

// Provides the TTL specified by the runtime. used for setting context durations.
// defaults to an hour. currently not fully implemented.
func TTL() time.Duration {
	return env.Duration(time.Hour, eg.EnvComputeTTL)
}

// Read the run ID from the environment
func RunID() string {
	return env.String("00000000-0000-0000-0000-000000000000", eg.EnvComputeRunID)
}

// returns the current user.
func User() *user.User {
	return userx.CurrentUserOrDefault(userx.Root())
}

// returns the absolute path to the workload directory of the module. this directory is the
// root directory of the workload.
//
// e.g.) WorkloadDirectory("foo", "bar") -> "/{eg.workload}/foo/bar"
func WorkloadDirectory(paths ...string) string {
	return filepath.Join(env.String(eg.DefaultWorkloadDirectory(), eg.EnvComputeWorkloadDirectory), filepath.Join(paths...))
}

// returns the absolute path to the cache directory, when arguments are provided they are joined
// joined with the cache directory.
//
// files stored in the cache directory are maintained between runs on a best effort basis.
// files prefixed with .eg are reserved for system use.
//
// e.g.) CacheDirectory("foo", "bar") -> "/{eg.cache}/foo/bar"
func CacheDirectory(paths ...string) string {
	return filepath.Join(env.String(os.TempDir(), eg.EnvComputeCacheDirectory, "CACHE_DIRECTORY"), filepath.Join(paths...))
}

// returns the absolute path to the runtime directory, when arguments are provided they are joined
// joined with the runime directory.
//
// files stored in the runtime directory are maintained for the duration of a workload. every module
// will be able to read the data stored in the runtime folder.
//
// e.g.) RuntimeDirectory("foo", "bar") -> "/{eg.runtime}/foo/bar"
func RuntimeDirectory(paths ...string) string {
	return filepath.Join(env.String(os.TempDir(), eg.EnvComputeRuntimeDirectory), filepath.Join(paths...))
}

// returns the absolute path to the workspace directory, when arguments are provided they are joined
// joined with the workspace directory.
//
// experimental directory - intended to be a directory for maintaining data for the lifetime of the workload.
//
// e.g.) WorkspaceDirectory("foo", "bar") -> "/{eg.workspace}/foo/bar"
func WorkspaceDirectory(paths ...string) string {
	return filepath.Join(env.String(eg.DefaultWorkspaceDirectory(), eg.EnvComputeWorkspaceDirectory), filepath.Join(paths...))
}

// returns the absolute path to the working directory of the module. this directory is the
// initial working directory of the workload and is used for cloning git repositories etc.
func WorkingDirectory(paths ...string) string {
	return filepath.Join(env.String(eg.DefaultWorkingDirectory(), eg.EnvComputeWorkingDirectory), filepath.Join(paths...))
}

// returns the absolute path to the ephemeral directory, when arguments are provided they are joined
// joined with the ephemeral directory.
//
// files stored in the ephemeral directory are maintained for the duration of a single module's execution.
// and is unique to that module.
//
// e.g.) EphemeralDirectory("foo", "bar") -> "/{eg.ephemeral}/foo/bar"
func EphemeralDirectory(paths ...string) string {
	return filepath.Join(os.TempDir(), filepath.Join(paths...))
}

// Extract a boolean formatted environment variable from the given keys
// returns the first valid result if none of the keys exist then the fallback is returned.
func Boolean(fallback bool, keys ...string) bool {
	return envx.Boolean(fallback, keys...)
}

// Extract a string formatted environment variable from the given keys.
// returns the first valid result if none of the keys exist then the fallback is returned.
func String(fallback string, keys ...string) string {
	return envx.String(fallback, keys...)
}

// Int retrieve a integer flag from the environment, checks each key in order
// first to parse successfully is returned.
func Int(fallback int, keys ...string) int {
	return envx.Int(fallback, keys...)
}

func Build() *envx.Builder {
	return envx.Build()
}
