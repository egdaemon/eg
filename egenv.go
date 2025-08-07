package eg

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tracex"
	"github.com/gofrs/uuid/v5"
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
	EnvPodmanSocket       = "EG_PODMAN_SOCKET"
	EnvContainerHost      = "CONTAINER_HOST"
	EnvEGSSHHost          = "EG_SSH_REVERSE_PROXY_HOST"
	EnvEGSSHProxyDisabled = "EG_SSH_REVERSE_PROXY_DISABLED"
	EnvEGSSHHostDefault   = "api.egdaemon.com:8090"
)

const (
	EnvP2PProxyDisabled = "EG_P2P_PROXY_DISABLED"
)

// Logging settings
const (
	EnvLogsInfo    = "EG_LOGS_INFO"    // enable logging for info statements. boolean, see strconv.ParseBool for valid values.
	EnvLogsDebug   = "EG_LOGS_DEBUG"   // enable logging for debug statements. boolean, see strconv.ParseBool for valid values.
	EnvLogsTrace   = "EG_LOGS_TRACE"   // enable logging for trace statements. boolean, see strconv.ParseBool for valid values.
	EnvLogsNetwork = "EG_LOGS_NETWORK" // enable logging for network requests. boolean, see strconv.ParseBool for valid values.
)

const (
	EnvCI                        = "CI"                                         // standard ci/cd environment variable flag
	EnvComputeTLSInsecure        = "EG_COMPUTE_TLS_INSECURE"                    // used to pass TLS insecure flag to container.
	EnvComputeLoggingVerbosity   = "EG_COMPUTE_LOG_VERBOSITY"                   // logging verbosity.
	EnvComputeModuleNestedLevel  = "EG_COMPUTE_MODULE_LEVEL"                    // number of nested levels the current module is running in.
	EnvComputeRootModule         = "EG_COMPUTE_ROOT_MODULE"                     // default is always false, but is set to true for the root module to bootstrap services
	EnvComputeRunID              = "EG_COMPUTE_RUN_ID"                          // run id for the compute workload
	EnvComputeAccountID          = "EG_COMPUTE_ACCOUNT_ID"                      // account id of the compute workload
	EnvComputeVCS                = "EG_COMPUTE_VCS_URI"                         // vcs uri for the compute workload
	EnvComputeTTL                = "EG_COMPUTE_TTL"                             // deadline for compute workload
	EnvComputeRootDirectory      = "EG_COMPUTE_ROOT_DIRECTORY"                  // root directory for workloads
	EnvComputeWorkingDirectory   = "EG_COMPUTE_WORKING_DIRECTORY"               // working directory for workloads
	EnvComputeCacheDirectory     = "EG_COMPUTE_CACHE_DIRECTORY"                 // cache directory for workloads
	EnvComputeRuntimeDirectory   = "EG_COMPUTE_RUNTIME_DIRECTORY"               // runtime directory for workloads
	EnvComputeWorkloadDirectory  = "EG_COMPUTE_WORKLOAD_DIRECTORY"              // workload directory for workloads
	EnvComputeWorkloadCapacity   = "EG_COMPUTE_WORKLOAD_CAPACITY"               // upper bound for the maximum number of workloads that can be run concurrently
	EnvComputeWorkloadTargetLoad = "EG_COMPUTE_WORKLOAD_TARGET_LOAD"            // upper bound for the maximum cpu load to target.
	EnvScheduleMaximumDelay      = "EG_COMPUTE_SCHEDULER_MAXIMUM_DELAY"         // maximum delay between checks for workloads.
	EnvScheduleSystemLoadFreq    = "EG_COMPUTE_SCHEDULER_SYSTEM_LOAD_FREQUENCY" // how frequently we measure system load, small enough we can saturate, high enough its not a burden.
	EnvPingMinimumDelay          = "EG_COMPUTE_PING_MINIMUM_DELAY"              // minimum delay for pings
	EnvComputeBin                = "EG_COMPUTE_BIN"                             // hotswap the binary, used for development testing
	EnvComputeBinAlt             = "EG_COMPUTE_BIN_ALTERNATE"                   // absolute path to an alternate binary to inject into the host environment when hotswapping.
	EnvComputeContainerImpure    = "EG_COMPUTE_C8S_IMPURE"                      // informs the container runner that the container depends on the repository being present.
	EnvComputeModuleSocket       = "EG_COMPUTE_MODULE_SOCKET"                   // socket providing functionality that is scoped to an individual module. primarily command execution.
	EnvComputeDefaultGroup       = "EG_COMPUTE_DEFAULT_GROUP"                   // override the group assigned to the user. mainly used by baremetal.
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
	EnvGitAuthHTTPPassword    = "EG_GIT_AUTH_HTTP_PASSWORD"
	EnvGitAuthHTTPUsername    = "EG_GIT_AUTH_HTTP_USERNAME"
)

const (
	EnvUnsafeCacheID         = "EG_UNSAFE_CACHE_ID"
	EnvUnsafeGitCloneEnabled = "EG_UNSAFE_GIT_CLONE_ENABLED"
)

const (
	WorkingDirectory  = "eg"
	CacheDirectory    = ".eg.cache"    // persistent cache between workloads
	RuntimeDirectory  = ".eg.runtime"  // runtime directory containing eg related files and sockets.
	WorkloadDirectory = ".eg.workload" // persistent shared directory for the duration of a single workload.
	ModuleDir         = "main.wasm.d"
	ModuleBin         = ".eg.module.wasm"
	BinaryBin         = "egbin"
	EnvironFile       = "environ.env"
	SocketControl     = "control.socket"
)

// generate unique module socket
func SocketModule() string {
	return fmt.Sprintf("module.%s.socket", errorsx.Must(uuid.NewV7()).String())
}

func DefaultModuleDirectory(rel ...string) string {
	return filepath.Join(filepath.Join(rel...), ".eg")
}

func DefaultCacheDirectory(rel ...string) string {
	return DefaultRootDirectory(CacheDirectory, filepath.Join(rel...))
}

func DefaultRuntimeDirectory(rel ...string) string {
	return DefaultMountRoot(RuntimeDirectory, filepath.Join(rel...))
}

func DefaultWorkingDirectory(rel ...string) string {
	return DefaultRootDirectory(WorkingDirectory, filepath.Join(rel...))
}

// default workload directory
func DefaultWorkloadDirectory(rel ...string) string {
	return DefaultRootDirectory(WorkloadDirectory, filepath.Join(rel...))
}

// default egd directory root. holds the directories accessible by egd.
func DefaultRootDirectory(rel ...string) string {
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

const (
	EnvExperimentalDisableHostNetwork = "EG_EXPERIMENTAL_DISABLE_HOST_NETWORK"
	EnvExperimentalBaremetal          = "EG_EXPERIMENTAL_BAREMETAL"
)
