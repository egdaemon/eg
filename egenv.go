package eg

import (
	"embed"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
)

var (
	apiHostDefault          = "https://api.egdaemon.com"
	consoleHostDefault      = "https://console.egdaemon.com"
	tlsinsecure             = "false"
	containerAPIHostDefault = ""
)

func EnvTLSInsecure() string {
	return tlsinsecure
}

func EnvAPIHostDefault() string {
	return apiHostDefault
}

func EnvConsoleHostDefault() string {
	return consoleHostDefault
}

func EnvContainerAPIHostDefault() string {
	return slicesx.FindOrZero(stringsx.Present, containerAPIHostDefault, apiHostDefault)
}

const (
	EnvContainerHost      = "CONTAINER_HOST"
	EnvEGSSHHost          = "EG_SSH_REVERSE_PROXY_HOST"
	EnvEGSSHProxyDisabled = "EG_SSH_REVERSE_PROXY_DISABLED"
	EnvEGSSHHostDefault   = "api.egdaemon.com:8090"
)

// Logging settings
const (
	EnvLogsInfo    = "EG_LOGS_INFO"    // enable logging for info statements. boolean, see strconv.ParseBool for valid values.
	EnvLogsDebug   = "EG_LOGS_DEBUG"   // enable logging for debug statements. boolean, see strconv.ParseBool for valid values.
	EnvLogsTrace   = "EG_LOGS_TRACE"   // enable logging for trace statements. boolean, see strconv.ParseBool for valid values.
	EnvLogsNetwork = "EG_LOGS_NETWORK" // enable logging for network requests. boolean, see strconv.ParseBool for valid values.
)

const (
	EnvComputeTLSInsecure       = "EG_COMPUTE_TLS_INSECURE"  // used to pass TLS insecure flag to container.
	EnvComputeLoggingVerbosity  = "EG_COMPUTE_LOG_VERBOSITY" // logging verbosity.
	EnvComputeModuleNestedLevel = "EG_COMPUTE_MODULE_LEVEL"  // number of nested levels the current module is running in.
	EnvComputeRootModule        = "EG_COMPUTE_ROOT_MODULE"   // default is always false, but is set to true for the root module to bootstrap services
	EnvComputeRunID             = "EG_COMPUTE_RUN_ID"        // run id for the compute workload
	EnvComputeAccountID         = "EG_COMPUTE_ACCOUNT_ID"    // account id of the compute workload
	EnvComputeVCS               = "EG_COMPUTE_VCS_URI"       // vcs uri for the compute workload
	EnvComputeTTL               = "EG_COMPUTE_TTL"           // deadline for compute workload
	EnvComputeContainerExec     = "EG_COMPUTE_EXEC_OPTIONS"  // CLI options for podman exec
	EnvComputeRootDirectory     = "EG_COMPUTE_ROOT_DIRECTORY"
	EnvComputeCacheDirectory    = "EG_COMPUTE_CACHE_DIRECTORY"
	EnvComputeRuntimeDirectory  = "EG_COMPUTE_RUNTIME_DIRECTORY"
	EnvComputeBin               = "EG_BIN"
	EnvScheduleMaximumDelay     = "EG_COMPUTE_SCHEDULER_MAXIMUM_DELAY"
	EnvPingMinimumDelay         = "EG_COMPUTE_PING_MINIMUM_DELAY"
	EnvComputeContainerImpure   = "EG_C8S_IMPURE"
)

const (
	EnvGitBaseVCS             = "EG_GIT_BASE_VCS"
	EnvGitBaseURI             = "EG_GIT_BASE_URI"
	EnvGitBaseRef             = "EG_GIT_BASE_REF"
	EnvGitBaseCommit          = "EG_GIT_BASE_COMMIT"
	EnvGitHeadVCS             = "EG_GIT_HEAD_VCS"
	EnvGitHeadURI             = "EG_GIT_HEAD_URI"
	EnvGitHeadRef             = "EG_GIT_HEAD_REF"
	EnvGitHeadCommit          = "EG_GIT_HEAD_COMMIT"
	EnvGitHeadCommitAuthor    = "EG_GIT_HEAD_COMMIT_AUTHOR"
	EnvGitHeadCommitEmail     = "EG_GIT_HEAD_COMMIT_EMAIL"
	EnvGitHeadCommitTimestamp = "EG_GIT_HEAD_COMMIT_TIMESTAMP"
)

const (
	EnvUnsafeCacheID         = "EG_UNSAFE_CACHE_ID"
	EnvUnsafeGitCloneEnabled = "EG_UNSAFE_GIT_CLONE_ENABLED"
)

func DefaultCacheDirectory(rel ...string) string {
	return filepath.Join("/", "opt", "eg.cache", filepath.Join(rel...))
}

func DefaultRuntimeDirectory(rel ...string) string {
	return filepath.Join("/", "opt", "eg.runtime", filepath.Join(rel...))
}

func DefaultRootDirectory(rel ...string) string {
	return filepath.Join("/", "opt", "eg", filepath.Join(rel...))
}

func DefaultTempDirectory(rel ...string) string {
	return filepath.Join("/", "opt", "eg.tmp", filepath.Join(rel...))
}

//go:embed DefaultContainerfile
var Embedded embed.FS

func PrepareRootContainer(cpath string) (err error) {
	var (
		c   fs.File
		dst *os.File
	)

	// log.Println("---------------------- Prepare Root Container Initiated ----------------------")
	// defer log.Println("---------------------- Prepare Root Container Completed ----------------------")

	log.Println("default container path", cpath)
	if c, err = Embedded.Open("DefaultContainerfile"); err != nil {
		return err
	}
	defer c.Close()

	if err = os.MkdirAll(filepath.Dir(cpath), 0700); err != nil {
		return err
	}

	if dst, err = os.OpenFile(cpath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600); err != nil {
		return err
	}

	if _, err = io.Copy(dst, c); err != nil {
		return err
	}

	return nil
}
