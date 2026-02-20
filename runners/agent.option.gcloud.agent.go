package runners

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
)

// configures the agent to use the current machines application default credentials.
func AgentOptionGcloudCredentials(ctx context.Context, envb *envx.Builder, path string) AgentOption {
	raw := errorsx.Must(os.ReadFile(path))
	envb.Var(eg.EnvUnsafeGcloudADCB64, base64.URLEncoding.EncodeToString(raw))

	// TODO: switch once eg.EnvUnsafeRemapDirectory is fully deployed.
	envb.Var("GOOGLE_APPLICATION_CREDENTIALS", eg.DefaultWorkloadDirectory("gcloud", "application_default_credentials.json"))

	errorsx.Never(envb.Append(eg.EnvUnsafeRemapDirectory, eg.DefaultWorkloadDirectory("gcloud"), ":")) // if this fails it means we've introduced a change to Append that can result in an error

	return AgentOptionVolumes(
		AgentMountOverlay(
			filepath.Dir(path),
			eg.DefaultMountRoot("gcloud"),
		),
	)
}
