package actl

type Bootstrap struct {
	Env    BootstrapEnv    `cmd:"" help:"bootstrap the a runner service environment file"`
	Module BootstrapModule `cmd:"" help:"bootstrap a local repository for use by eg"`
}
