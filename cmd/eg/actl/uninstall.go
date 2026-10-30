package actl

type Uninstall struct {
	Module UninstallModule `cmd:"" help:"remove the eg module from a local repository"`
}
