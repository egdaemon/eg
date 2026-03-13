package runners

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/userx"
)

// configures a local machine run to connect to the users gpg agent.
func AgentOptionLocalGPGAgent(ctx context.Context, envb *envx.Builder) AgentOption {
	// https://wiki.gnupg.org/AgentForwarding
	// https://github.com/gpg/gnupg/blob/master/tools/gpg-connect-agent.c
	// https://blog.packagecloud.io/how-to-gpg-sign-and-verify-deb-packages-and-apt-repositories/
	upstream := userx.DefaultRuntimeDirectory("gnupg", "S.gpg-agent")

	gnupghome, err := userx.HomeDirectory(".gnupg")
	if err != nil {
		log.Println("unable to check if gpg config is available", err)
		return AgentOptionNoop
	}

	return agentOptionLocalGPGAgent(ctx, envb, upstream, gnupghome)
}

func agentOptionLocalGPGAgent(ctx context.Context, envb *envx.Builder, agentsock string, gnupghome string) AgentOption {
	if _, err := os.Stat(agentsock); fsx.ErrIsNotExist(err) != nil {
		log.Println("gpg agent is not available", err)
		return AgentOptionNoop
	} else if err != nil {
		log.Println("unable to check if gpg agent is available", err)
		return AgentOptionNoop
	}

	if _, err := os.Stat(gnupghome); fsx.ErrIsNotExist(err) != nil {
		log.Println("gpg config is not available", err)
		return AgentOptionNoop
	} else if err != nil {
		log.Println("unable to check if gpg config is available", err)
		return AgentOptionNoop
	}

	envb.Var("GNUPGHOME", eg.DefaultMountRoot(".gnupg"))

	runtimedir := filepath.Dir(agentsock)
	return AgentOptionVolumes(
		AgentMountOverlay(
			gnupghome,
			eg.DefaultMountRoot(".gnupg"),
		),
		AgentMountReadWrite(
			agentsock,
			eg.DefaultMountRoot(".gnupg", "S.gpg-agent"),
		),
		AgentMountReadWrite(
			filepath.Join(runtimedir, "S.dirmngr"),
			eg.DefaultMountRoot(".gnupg", "S.dirmngr"),
		),
		AgentMountReadWrite(
			filepath.Join(runtimedir, "S.keyboxd"),
			eg.DefaultMountRoot(".gnupg", "S.keyboxd"),
		),
	)
}

// NotImplemented, but plan is to connect a runner to upstream services for gpg signing.
func AgentOptionGPGAgent(desc ...string) AgentOption {
	return AgentOptionNoop
}
