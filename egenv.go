package eg

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
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
	EnvEGSSHSeed          = "EG_SSH_KEY_SEED"
	EnvEGSSHHost          = "EG_SSH_REVERSE_PROXY_HOST"
	EnvEGSSHProxyDisabled = "EG_SSH_REVERSE_PROXY_DISABLED"
	EnvEGSSHHostDefault   = "api.egdaemon.com:8090"
	EnvRuntimeDirectory   = "RUNTIME_DIRECTORY" // standard environment variable for runtime directories.
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
	EnvGithubToken                   = "GH_TOKEN"                                   // standard environment variable for github token
	EnvCI                            = "CI"                                         // standard ci/cd environment variable flag
	EnvComputeTLSInsecure            = "EG_COMPUTE_TLS_INSECURE"                    // used to pass TLS insecure flag to container.
	EnvComputeLoggingVerbosity       = "EG_COMPUTE_LOG_VERBOSITY"                   // logging verbosity.
	EnvComputeModuleNestedLevel      = "EG_COMPUTE_MODULE_LEVEL"                    // number of nested levels the current module is running in.
	EnvComputeRootModule             = "EG_COMPUTE_ROOT_MODULE"                     // default is always false, but is set to true for the root module to bootstrap services
	EnvComputeRunID                  = "EG_COMPUTE_RUN_ID"                          // run id for the compute workload
	EnvComputeAccountID              = "EG_COMPUTE_ACCOUNT_ID"                      // account id of the compute workload
	EnvComputeVCS                    = "EG_COMPUTE_VCS_URI"                         // vcs uri for the compute workload
	EnvComputeTTL                    = "EG_COMPUTE_TTL"                             // deadline for compute workload
	EnvComputeWorkload               = "EG_COMPUTE_WORKLOAD"                        // entry path of the workload being run.
	EnvComputeWorkloadName           = "EG_COMPUTE_WORKLOAD_NAME"                   // module the workload runs (folder name within the module directory), blank for the default workload.
	EnvComputeInitiatedTimestamp     = "EG_COMPUTE_INITIATED_TIMESTAMP"             // RFC3339Nano UTC instant the billed run duration starts at.
	EnvComputeArch                   = "EG_COMPUTE_HOST_ARCH"                       // host machine cpu architecture
	EnvComputeOS                     = "EG_COMPUTE_HOST_OS"                         // host machine operating system
	EnvComputeWorkloadDirectory      = "EG_COMPUTE_WORKLOAD_DIRECTORY"              // root directory for workloads
	EnvComputeWorkingDirectory       = "EG_COMPUTE_WORKING_DIRECTORY"               // working directory for workloads
	EnvComputeCacheDirectory         = "EG_COMPUTE_CACHE_DIRECTORY"                 // cache directory for workloads
	EnvComputeRuntimeDirectory       = "EG_COMPUTE_RUNTIME_DIRECTORY"               // runtime directory for workloads
	EnvComputeWorkspaceDirectory     = "EG_COMPUTE_WORKSPACE_DIRECTORY"             // workspace directory for workloads
	EnvComputeWorkloadCapacity       = "EG_COMPUTE_WORKLOAD_CAPACITY"               // upper bound for the maximum number of workloads that can be run concurrently
	EnvComputeCompileCapacity        = "EG_COMPUTE_COMPILE_CAPACITY"                // upper bound for the maximum number of workloads that can be compiled concurrently
	EnvComputeWorkloadTargetLoad     = "EG_COMPUTE_WORKLOAD_TARGET_LOAD"            // upper bound for the maximum cpu load to target.
	EnvScheduleMaximumDelay          = "EG_COMPUTE_SCHEDULER_MAXIMUM_DELAY"         // maximum delay between checks for workloads.
	EnvScheduleSystemLoadFreq        = "EG_COMPUTE_SCHEDULER_SYSTEM_LOAD_FREQUENCY" // how frequently we measure system load, small enough we can saturate, high enough its not a burden.
	EnvPingMinimumDelay              = "EG_COMPUTE_PING_MINIMUM_DELAY"              // minimum delay for pings
	EnvComputeBin                    = "EG_COMPUTE_BIN"                             // hotswap the binary, used for development testing
	EnvComputeBinAlt                 = "EG_COMPUTE_BIN_ALTERNATE"                   // absolute path to an alternate binary to inject into the host environment when hotswapping.
	EnvComputeContainerImpure        = "EG_COMPUTE_C8S_IMPURE"                      // informs the container runner that the container depends on the repository being present.
	EnvComputeGPU                    = "EG_COMPUTE_GPU"                             // enable gpu support for the compute workload, propagated to nested module containers.
	EnvComputeModuleSocket           = "EG_COMPUTE_MODULE_SOCKET"                   // socket providing functionality that is scoped to an individual module. primarily command execution.
	EnvComputeDefaultGroup           = "EG_COMPUTE_DEFAULT_GROUP"                   // override the group assigned to the user. mainly used by baremetal.
	EnvComputeAPIEnabled             = "EG_COMPUTE_API_ENABLED"                     // gates the runner's push HTTP surface (POST /c/upload, POST /c/enqueue) and whether the runner advertises itself as uploadable; enabled by default, see the daemon's --uploadable flag.
	EnvComputeGuestMigrationsEnabled = "EG_COMPUTE_GUEST_MIGRATIONS_ENABLED"        // gates whether guests migrate the analytics database over its socket before writing to it (egdb.OpenMigrate); enabled by default.
	EnvComputeProfileMode            = "EG_COMPUTE_PROFILE_MODE"                    // profile mode (cpu,heap,mem,allocs,block) for module runs.
	EnvComputeLLMEndpoint            = "EG_COMPUTE_LLM_ENDPOINT"                    // override the base url (http(s):// or unix:///path/to.sock) of the llm server used by egllm, the server must provide llama.cpp's OpenAI-compatible api. defaults to llama.cpp's, which is managed for the duration of the workload.
	EnvBuildCommit                   = "EG_COMMIT"                                  // build arg for the root container carrying the commit of the eg build, busts the container cache each release.
	EnvRestartOnUpgrade              = "EG_RESTART_ON_UPGRADE"                      // restart the daemon when the eg package is upgraded: auto (default), yes or no. read from daemon.env by the package's postinst.
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

// IsDefaultBranchBuild reports whether the head and base commits recorded in
// environ are identical, which only happens for default-branch builds (see
// vcsevents push-event handling, where Head and Base are built from the same
// Reference) and local runs performed directly on the base branch.
//
// implemented without internal/envx to avoid an import cycle -- envx itself
// depends on this package.
func IsDefaultBranchBuild(environ ...string) bool {
	lookup := func(key string) string {
		prefix := key + "="
		for _, kv := range environ {
			if v, ok := strings.CutPrefix(kv, prefix); ok {
				return v
			}
		}
		return ""
	}

	head := lookup(EnvGitHeadCommit)
	base := lookup(EnvGitBaseCommit)
	return head != "" && head == base
}

const (
	EnvGoogleApplicationCredentials   = "GOOGLE_APPLICATION_CREDENTIALS"         // path to the application default credentials used by google clients and gcloud.
	EnvCloudSDKCredentialFileOverride = "CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE" // makes gcloud commands use the credentials in the file, set when the workload was provided credentials.
	EnvGCPWorkloadIdentityEnabled     = "EG_GCP_WORKLOAD_IDENTITY_ENABLED"       // set by whatever generates the workload (uploads, schedules) when it needs gcp credentials issued to it by its runner.
)

const (
	EnvUnsafeCacheID         = "EG_UNSAFE_CACHE_ID"
	EnvUnsafeGitCloneEnabled = "EG_UNSAFE_GIT_CLONE_ENABLED"
	EnvUnsafeRemapDirectory  = "EG_UNSAFE_REMAP_DIRECTORY"                    // list of directories to remap, seperated by ':'.
	EnvUnsafeGcloudADCB64    = "EG_UNSAFE_GOOGLE_APPLICATION_CREDENTIALS_B64" // application default credentials, base64 encoded with the url alphabet (base64.URLEncoding). temporary hack while we deploy remap directory
)

// TODO: RuntimeDirectory, rename the value to ".eg.d". runner only change now that modules read EnvComputeRuntimeDirectory
// (see egunsafe.RuntimeDirectory). modules built before that read this constant, so keep the legacy path
// working for a window: mount the host dir a second time at /eg.mnt/.eg.runtime (runners/queue.go) and symlink
// {root}/.eg.runtime -> .eg.d (workspaces/workspace.go). plain symlinks inside the container and hard links do not work.
const (
	DefaultUsername    = "egd" // workload username
	WorkingDirectory   = "eg"
	DefaultWorkload    = "default" // name of the default workload, used where the entry path is blank.
	MountDirectory     = "eg.mnt"
	WorkloadDirectory  = ".eg.workload"
	CacheDirectory     = ".eg.cache"     // persistent cache between workloads
	RuntimeDirectory   = ".eg.runtime"   // runtime directory containing eg related files and sockets.
	WorkspaceDirectory = ".eg.workspace" // persistent shared directory for the duration of a single workload.
	ModuleDir          = "main.wasm.d"
	ModuleBin          = ".eg.module.wasm"
	BinaryBin          = "egbin"
	EnvironFile        = "environ.env"
	SocketControl      = "control.socket"
	SocketAnalytics    = "analytics.socket"
	SocketAnalyticsPG  = ".s.PGSQL.5432" // postgres wire frontend of the analytics database, the name libpq style clients derive from a directory.
)

// generate unique module socket
func SocketModule() string {
	return fmt.Sprintf("module.%s.socket", errorsx.Must(uuid.NewV7()).String())
}

func DefaultModuleDirectory(rel ...string) string {
	return filepath.Join(filepath.Join(rel...), ".eg")
}

// AnalyticsCachePath is the path relative to the cache directory of the analytics database of the last successful
// run of the workload (module name), blank workloads are the default workload. the workload is hashed so any
// name is a safe directory name.
// e.g.) AnalyticsCachePath("foo") -> ".eg/db/{md5x.String("foo")}/latest.db"
func AnalyticsCachePath(workload string) string {
	return filepath.Join(DefaultModuleDirectory(), "db", md5x.String(stringsx.First(workload, DefaultWorkload)), "latest.db")
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

// default workspace directory
func DefaultWorkspaceDirectory(rel ...string) string {
	return DefaultRootDirectory(WorkspaceDirectory, filepath.Join(rel...))
}

// default egd directory root. holds the directories accessible by egd.
func DefaultRootDirectory(rel ...string) string {
	return filepath.Join("/", "workload", filepath.Join(rel...))
}

// root mount location, all volumes are initially mounted here.
// then they're rebound to grant the unprivileged users access.
func DefaultMountRoot(rel ...string) string {
	return filepath.Join("/", MountDirectory, filepath.Join(rel...))
}

func ModuleMount() string {
	return DefaultMountRoot(RuntimeDirectory, ModuleBin)
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
	EnvExperimentalBindMount          = "EG_EXPERIMENTAL_BIND_MOUNT"
)
