package eggcp

import (
	"context"
	"net/http"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/wasinet/wasinet"
)

// temporary hack until remap directory functionality is working.
func CredentialsHack(ctx context.Context, o eg.Op) error {
	http.DefaultTransport = wasinet.InsecureHTTP()

	encoded := envx.String("", _eg.EnvUnsafeGcloudADCB64)
	if encoded == "" {
		return nil
	}

	return shell.Run(
		ctx,
		shell.Newf("echo %s | tr -- '-_' '+/' | base64 -d -i | install -D -m 600 /dev/stdin ~/.config/gcloud/application_default_credentials.json", encoded),
		shell.Newf("echo %s | tr -- '-_' '+/' | base64 -d -i | install -D -m 600 /dev/stdin %s", encoded, egenv.WorkloadDirectory("gcloudhack", "application_default_credentials.json")),
	)
}
