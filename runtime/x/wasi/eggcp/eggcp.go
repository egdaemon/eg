package eggcp

import (
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// Credentials points gcloud at the application default credentials in the file adc. gcloud commands (storage, compute, ...)
// only use credentials of their own account and ignore the application default credentials, which the google client
// libraries use, unless auth/credential_file_override is set. the runner does this for the credentials it provides to
// the workload, use this for credentials installed anywhere else.
func Credentials(adc string) eg.OpFn {
	return shell.Op(
		shell.Newf("gcloud config set auth/credential_file_override %s", adc),
	)
}

// Debug returns a command that prints how gcloud is configured for the user the command runs as: the user, home directory,
// the contents of ~/.config/gcloud, the google and gcloud environment, the credentialed accounts and every gcloud property. it never aborts the workload.
// c provides the environment and user the command runs with, e.g. shell.Run(ctx, eggcp.Debug(console)).
func Debug(c shell.Command) shell.Command {
	return c.New(`id; echo HOME=${HOME}; tree -a -p -u -s ${HOME}/.config/gcloud; env | grep -E '^(CLOUDSDK|GOOGLE)'; gcloud auth list; gcloud config list --all`).Debug().Lenient(true)
}
