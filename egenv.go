package eg

import (
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
	EnvComputeRootModule    = "EG_COMPUTE_ROOT_MODULE" // default is always false, but is set to true for the root module to bootstrap services
	EnvComputeRunID         = "EG_COMPUTE_RUN_ID"      // run id for the compute workload
	EnvComputeTTL           = "EG_COMPUTE_TTL"         // deadline for compute workload
	EnvComputeContainerExec = "EG_EXEC_OPTIONS"        // CLI options for podman exec
	EnvScheduleMaximumDelay = "EG_COMPUTE_SCHEDULER_MAXIMUM_DELAY"
	EnvPingMinimumDelay     = "EG_COMPUTE_PING_MINIMUM_DELAY"
)
