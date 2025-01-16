package eg

import (
	"embed"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tracex"
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
	EnvComputeTLSInsecure        = "EG_COMPUTE_TLS_INSECURE"                    // used to pass TLS insecure flag to container.
	EnvComputeLoggingVerbosity   = "EG_COMPUTE_LOG_VERBOSITY"                   // logging verbosity.
	EnvComputeModuleNestedLevel  = "EG_COMPUTE_MODULE_LEVEL"                    // number of nested levels the current module is running in.
	EnvComputeRootModule         = "EG_COMPUTE_ROOT_MODULE"                     // default is always false, but is set to true for the root module to bootstrap services
	EnvComputeRunID              = "EG_COMPUTE_RUN_ID"                          // run id for the compute workload
	EnvComputeAccountID          = "EG_COMPUTE_ACCOUNT_ID"                      // account id of the compute workload
	EnvComputeVCS                = "EG_COMPUTE_VCS_URI"                         // vcs uri for the compute workload
	EnvComputeTTL                = "EG_COMPUTE_TTL"                             // deadline for compute workload
	EnvComputeContainerExec      = "EG_COMPUTE_EXEC_OPTIONS"                    // CLI options for podman exec
	EnvComputeWorkingDirectory   = "EG_COMPUTE_ROOT_DIRECTORY"                  // root working directory for workloads
	EnvComputeCacheDirectory     = "EG_COMPUTE_CACHE_DIRECTORY"                 // cache directory for workloads
	EnvComputeRuntimeDirectory   = "EG_COMPUTE_RUNTIME_DIRECTORY"               // runtime directory for workloads
	EnvComputeWorkloadCapacity   = "EG_COMPUTE_WORKLOAD_CAPACITY"               // upper bound for the maximum number of workloads that can be run concurrently
	EnvComputeWorkloadTargetLoad = "EG_COMPUTE_WORKLOAD_TARGET_LOAD"            // upper bound for the maximum number of workloads that can be run concurrently
	EnvScheduleMaximumDelay      = "EG_COMPUTE_SCHEDULER_MAXIMUM_DELAY"         // maximum delay between scans for workloads
	EnvScheduleSystemLoadFreq    = "EG_COMPUTE_SCHEDULER_SYSTEM_LOAD_FREQUENCY" // how frequently we measure system load, small enough we can saturate, high enough its not a burden.
	EnvPingMinimumDelay          = "EG_COMPUTE_PING_MINIMUM_DELAY"              // minimum delay for pings
	EnvComputeBin                = "EG_BIN"
	EnvComputeContainerImpure    = "EG_C8S_IMPURE"
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

const (
	WorkingDirectory = "eg"
	CacheDirectory   = ".eg.cache"
	RuntimeDirectory = ".eg.runtime"
	ModuleBin        = ".eg.module.wasm"
	BinaryBin        = "egbin"
)

func DefaultModuleDirectory() string {
	return ".eg"
}

func DefaultCacheDirectory(rel ...string) string {
	return DefaultWorkloadRoot(CacheDirectory, filepath.Join(rel...))
}

func DefaultRuntimeDirectory(rel ...string) string {
	return DefaultWorkloadRoot(RuntimeDirectory, filepath.Join(rel...))
}

func DefaultWorkingDirectory(rel ...string) string {
	return DefaultWorkloadRoot(WorkingDirectory, filepath.Join(rel...))
}

func DefaultWorkloadRoot(rel ...string) string {
	return filepath.Join("/", "workload", filepath.Join(rel...))
}

// root mount location, all volumes are initially mounted here.
// then they're rebound to grant the unprivileged users access.
func DefaultMountRoot(rel ...string) string {
	return filepath.Join("/", "eg.mnt", filepath.Join(rel...))
}

//go:embed DefaultContainerfile
var Embedded embed.FS

func PrepareRootContainer(cpath string) (err error) {
	var (
		c   fs.File
		dst *os.File
	)

	tracex.Println("---------------------- Prepare Root Container Initiated ----------------------")
	tracex.Println("default container path", cpath)
	defer tracex.Println("---------------------- Prepare Root Container Completed ----------------------")
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
