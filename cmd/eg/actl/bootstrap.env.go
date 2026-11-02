package actl

type BootstrapEnv struct {
	Runner BootstrapEnvRunner `cmd:"" help:"bootstrap the a runner service environment file"`
	Daemon BootstrapEnvDaemon `cmd:"" help:"bootstrap the a daemon service environment file"`
	Git    BootstrapEnvGit    `cmd:"" help:"bootstrap git's user.name and user.email from your eg identity"`
}
