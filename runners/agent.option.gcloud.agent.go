package runners

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
)

// configures the agent to use the current machines application default credentials.
// the credentials are provided to the workload in its environment, the runner installs them as the
// application default credentials of the user, see gcpx.InstallCredentials.
func AgentOptionGcloudCredentials(ctx context.Context, envb *envx.Builder, path string) AgentOption {
	raw := errorsx.Must(os.ReadFile(path))
	envb.Var(eg.EnvUnsafeGcloudADCB64, base64.URLEncoding.EncodeToString(raw))

	return AgentOptionNoop
}
