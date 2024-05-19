package eg

const (
	EnvEGAPIHost        = "EG_API_HOST"
	EnvEGAPIHostDefault = "https://api.egdaemon.com"
	// EnvEGAPIHostDefault = "https://localhost:8081"
)

const (
	EnvEGSSHHost        = "EG_SSH_HOST"
	EnvEGSSHHostDefault = "api.egdaemon.com:8090"
	// EnvEGSSHHostDefault = "localhost:8090"
)

const (
	EnvEGConsoleHost        = "EG_CONSOLE_HOST"
	EnvEGConsoleHostDefault = "https://console.egdaemon.com"
	// EnvEGConsoleHostDefault = "https://localhost:8080"
)

// Logging settings
const (
	EnvLogsInfo    = "EG_LOGS_INFO"    // enable logging for info statements. boolean, see strconv.ParseBool for valid values.
	EnvLogsDebug   = "EG_LOGS_DEBUG"   // enable logging for debug statements. boolean, see strconv.ParseBool for valid values.
	EnvLogsTrace   = "EG_LOGS_TRACE"   // enable logging for trace statements. boolean, see strconv.ParseBool for valid values.
	EnvLogsNetwork = "EG_LOGS_NETWORK" // enable logging for network requests. boolean, see strconv.ParseBool for valid values.
)

const (
	EnvScheduleMaximumDelay = "EG_COMPUTE_SCHEDULER_MAXIMUM_DELAY"
)
