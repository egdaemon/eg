package eggcp

import (
	"context"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// temporary hack until remap directory functionality is working.
func CredentialsHack(ctx context.Context, o eg.Op) error {
	encoded := envx.String("", _eg.EnvUnsafeGcloudADCB64)
	if encoded == "" {
		return nil
	}

	return shell.Run(
		ctx,
		shell.Newf("echo %s | tr -- '-_' '+/' | base64 -d -i | install -D -m 600 /dev/stdin ~/.config/gcloud/application_default_credentials.json", encoded),
		shell.Newf("echo %s | tr -- '-_' '+/' | base64 -d -i | install -D -m 600 /dev/stdin %s", encoded, egenv.WorkloadDirectory("gcloud", "application_default_credentials.json")),
		shell.Newf("which gcloud && gcloud config set auth/credential_file_override %s", egenv.WorkloadDirectory("gcloud", "application_default_credentials.json")),
	)
}
