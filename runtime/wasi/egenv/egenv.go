package egenv

import (
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/runtime/wasi/env"
)

// Provides the TTL specified by the runtime. used for setting context durations.
// defaults to an hour. currently not fully implemented.
func TTL() time.Duration {
	return env.Duration(time.Hour, eg.EnvComputeTTL)
}

// CloneEnabled reports whether the repository is cloned from, and so pushed to, a remote. false for `eg compute local`,
// which works on the local checkout where there is nothing to push to.
func CloneEnabled() bool {
	return env.Boolean(true, eg.EnvUnsafeGitCloneEnabled)
}

// Read the run ID from the environment
func RunID() string {
	return env.String("00000000-0000-0000-0000-000000000000", eg.EnvComputeRunID)
}

// Read the account ID from the environment
func AccountID() string {
	return env.String("00000000-0000-0000-0000-000000000000", eg.EnvComputeAccountID)
}

var initiated = sync.OnceValue(func() time.Time {
	ts, err := time.Parse(time.RFC3339Nano, env.String("", eg.EnvComputeInitiatedTimestamp))
	if err != nil {
		log.Println("unable to parse", eg.EnvComputeInitiatedTimestamp, "falling back to the current time", err)
		return time.Now()
	}

	return ts
})

// Initiated provides the instant the billed run duration started at. returns time.Now()
// as an approximation: when the runtime did not provide it, the time of the first call is used.
// the environment is only parsed once.
func Initiated() time.Time {
	return initiated()
}

// Read the entry path of the workload from the environment, empty when the runtime did not provide it.
func Workload() string {
	return env.String("", eg.EnvComputeWorkload)
}

// Read the name of the workload (the module it runs, i.e. the folder name within the module directory)
// from the environment, empty for the default workload (see eg.DefaultWorkload).
func WorkloadName() string {
	return env.String("", eg.EnvComputeWorkloadName)
}

// Pattern substitutes values from the environment into the provided pattern.
//
// %eg.run.id%       -> the run id of the compute workload, see RunID.
// %eg.account.id%   -> the account id of the compute workload, see AccountID.
// %eg.workload%     -> the name of the module the workload runs, see WorkloadName.
// %eg.host.arch%    -> the cpu architecture of the host machine.
// %eg.host.os%      -> the operating system of the host machine.
func Pattern(s string) string {
	s = strings.ReplaceAll(s, "%eg.run.id%", RunID())
	s = strings.ReplaceAll(s, "%eg.account.id%", AccountID())
	s = strings.ReplaceAll(s, "%eg.workload%", WorkloadName())
	s = strings.ReplaceAll(s, "%eg.host.arch%", env.String("", eg.EnvComputeArch))
	s = strings.ReplaceAll(s, "%eg.host.os%", env.String("", eg.EnvComputeOS))
	return s
}

// User returns the current user.
func User() *user.User {
	return userx.CurrentUserOrDefault(userx.Root())
}

// dir joins paths onto the first environment key that is set, falling back to fallback when none are.
func dir(fallback string, paths []string, keys ...string) string {
	return filepath.Join(env.String(fallback, keys...), filepath.Join(paths...))
}

// DirectoryCache returns the absolute path to the cache directory, EG_COMPUTE_CACHE_DIRECTORY or the systemd
// CACHE_DIRECTORY, falling back to os.TempDir() when neither is set.
//
// lifecycle: survives across workloads on a best effort basis. entries prefixed with .eg are reserved for system use.
//
// paths are joined onto the directory.
// e.g.) DirectoryCache("foo", "bar") -> "/workload/.eg.cache/foo/bar"
func DirectoryCache(paths ...string) string {
	return dir(os.TempDir(), paths, eg.EnvComputeCacheDirectory, userx.EnvCacheDirectory)
}

// CacheDirectory returns the absolute path to the cache directory, joined with paths when provided.
//
// Deprecated: use DirectoryCache.
func CacheDirectory(paths ...string) string {
	return DirectoryCache(paths...)
}

// RuntimeDirectory returns the absolute path to the runtime directory, EG_COMPUTE_RUNTIME_DIRECTORY, falling back
// to os.TempDir() when it is not set. the directory holds the files eg itself uses, e.g.) sockets.
//
// lifecycle: lives for the duration of a workload and is readable by every module.
//
// paths are joined onto the directory.
// e.g.) RuntimeDirectory("foo", "bar") -> "/eg.mnt/.eg.runtime/foo/bar"
//
// Deprecated: the directory is owned by eg and is not for use by workloads, use egunsafe.RuntimeDirectory if it is
// required.
func RuntimeDirectory(paths ...string) string {
	return dir(os.TempDir(), paths, eg.EnvComputeRuntimeDirectory)
}

// WorkspaceDirectory returns the absolute path to the workspace directory, EG_COMPUTE_WORKSPACE_DIRECTORY, falling
// back to eg.DefaultWorkspaceDirectory() when it is not set.
//
// lifecycle: shared by every module and lives for the duration of a single workload.
//
// paths are joined onto the directory.
// e.g.) WorkspaceDirectory("foo", "bar") -> "/workload/.eg.workspace/foo/bar"
func WorkspaceDirectory(paths ...string) string {
	return dir(eg.DefaultWorkspaceDirectory(), paths, eg.EnvComputeWorkspaceDirectory)
}

// DirectoryWorking returns the absolute path to the working directory, EG_COMPUTE_WORKING_DIRECTORY, falling back
// to eg.DefaultWorkingDirectory() when it is not set. this is the initial working directory of the workload
// and is where the repository is cloned.
//
// lifecycle: lives for the duration of a single workload.
//
// paths are joined onto the directory.
// e.g.) DirectoryWorking("foo", "bar") -> "/workload/eg/foo/bar"
func DirectoryWorking(paths ...string) string {
	return dir(eg.DefaultWorkingDirectory(), paths, eg.EnvComputeWorkingDirectory)
}

// WorkingDirectory returns the absolute path to the working directory, joined with paths when provided.
//
// Deprecated: use DirectoryWorking.
func WorkingDirectory(paths ...string) string {
	return DirectoryWorking(paths...)
}

// PathRelative reports paths within dir relative to it. relative paths and paths outside dir are returned as is.
func PathRelative(dir string, path string) string {
	if !filepath.IsAbs(path) {
		return path
	}

	rel, err := filepath.Rel(dir, path)
	if err != nil || !filepath.IsLocal(rel) {
		return path
	}

	return rel
}

// PathRelativeWorkspace reports paths within the workspace directory relative to it, so they hold wherever the workspace is
// located, e.g.) a git worktree. relative paths and paths outside the workspace are returned as is.
func PathRelativeWorkspace(path string) string {
	return PathRelative(WorkspaceDirectory(), path)
}

// PathRelativeWorking reports paths within the working directory relative to it, so they hold wherever the working directory
// is located. relative paths and paths outside the working directory are returned as is.
func PathRelativeWorking(path string) string {
	return PathRelative(DirectoryWorking(), path)
}

// PathRelativeCache reports paths within the cache directory relative to it. relative paths and paths outside the
// cache directory are returned as is.
func PathRelativeCache(path string) string {
	return PathRelative(DirectoryCache(), path)
}

// PathRelativeEphemeral reports paths within the ephemeral directory relative to it. relative paths and paths outside the
// ephemeral directory are returned as is.
func PathRelativeEphemeral(path string) string {
	return PathRelative(DirectoryTemp(), path)
}

// DirectoryTemp returns the absolute path to the temporary directory, which is os.TempDir(). every module
// is given its own temporary directory, so it is unique to the module.
//
// lifecycle: lives for the duration of a single module's execution.
//
// paths are joined onto the directory.
// e.g.) DirectoryTemp("foo", "bar") -> "/tmp/foo/bar"
func DirectoryTemp(paths ...string) string {
	return dir(os.TempDir(), paths)
}

// EphemeralDirectory returns the absolute path to the ephemeral directory, joined with paths when provided.
//
// Deprecated: use DirectoryTemp.
func EphemeralDirectory(paths ...string) string {
	return DirectoryTemp(paths...)
}

// Extract a time.Duration formatted environment variable from the given keys
// returns the first valid result if none of the keys exist then the fallback is returned.
func Duration(fallback time.Duration, keys ...string) time.Duration {
	return envx.Duration(fallback, keys...)
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

// Base64 read value as a base64 encoded string, the standard alphabet is attempted first and then the url alphabet.
func Base64(fallback []byte, keys ...string) []byte {
	return envx.Base64(fallback, keys...)
}

// Base64URL read value as a url base64 encoded string
func Base64URL(fallback []byte, keys ...string) []byte {
	return envx.Base64URL(fallback, keys...)
}

// Int retrieve a integer flag from the environment, checks each key in order
// first to parse successfully is returned.
func Int(fallback int, keys ...string) int {
	return envx.Int(fallback, keys...)
}

func Build() *envx.Builder {
	return envx.Build()
}

func MustEnviron(env *envx.Builder) []string {
	return errorsx.Must(env.Environ())
}
