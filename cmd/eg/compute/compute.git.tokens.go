package compute

import (
	"context"
	"log"
	"strings"

	"github.com/egdaemon/eg/internal/execx"
	"github.com/logrusorgru/aurora"
)

// githubToken attempts to auto-detect a github auth token for local
// compute runs by shelling out to the gh cli when the repository's
// canonical remote uri looks like github.com. returns "" (a no-op for
// envx.Builder.Var) if the remote isn't github or the user isn't
// authenticated. logs a warning, consistent with other experimental
// local-compute functionality (see runners.AgentOptionGPU), when gh
// isn't installed or the token lookup fails.
func githubToken(ctx context.Context, canonicaluri string) string {
	if !strings.Contains(canonicaluri, "github.com") {
		return ""
	}

	if _, err := execx.LookPath("gh"); err != nil {
		log.Println(aurora.NewAurora(true).Yellow("warning: github remote detected but gh cli was not found, skipping automatic GH_TOKEN detection"))
		return ""
	}

	token, err := execx.String(ctx, "gh", "auth", "token")
	if err != nil {
		log.Println(aurora.NewAurora(true).Yellow("warning: unable to retrieve github token via gh cli, skipping automatic GH_TOKEN detection"), err)
		return ""
	}

	return strings.TrimSpace(token)
}
