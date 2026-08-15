package actl

type BootstrapEnv struct {
	Runner BootstrapEnvRunner `cmd:"" help:"bootstrap the a runner service environment file"`
	Daemon BootstrapEnvDaemon `cmd:"" help:"bootstrap the a daemon service environment file"`
}
