package runners

import (
	"context"
	"log"
	"os"

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

	if _, err := os.Stat(upstream); fsx.ErrIsNotExist(err) != nil {
		log.Println("gpg agent is not available", err)
		return AgentOptionNoop
	} else if err != nil {
		log.Println("unable to check if gpg agent is available", err)
		return AgentOptionNoop
	}

	gnupghome, err := userx.HomeDirectory(".gnupg")
	if err != nil {
		log.Println("unable to check if gpg agent is available", err)
		return AgentOptionNoop
	}

	envb.Var("GNUPGHOME", eg.DefaultMountRoot(eg.RuntimeDirectory, ".gnupg"))
	return AgentOptionVolumes(
		AgentMountOverlay(
			gnupghome,
			eg.DefaultMountRoot(eg.RuntimeDirectory, ".gnupg"),
		),
		AgentMountReadWrite(
			userx.DefaultRuntimeDirectory("gnupg", "S.gpg-agent"),
			eg.DefaultMountRoot(eg.RuntimeDirectory, ".gnupg", "S.gpg-agent"),
		),
		AgentMountReadWrite(
			userx.DefaultRuntimeDirectory("gnupg", "S.dirmngr"),
			eg.DefaultMountRoot(eg.RuntimeDirectory, ".gnupg", "S.dirmngr"),
		),
		AgentMountReadWrite(
			userx.DefaultRuntimeDirectory("gnupg", "S.keyboxd"),
			eg.DefaultMountRoot(eg.RuntimeDirectory, ".gnupg", "S.keyboxd"),
		),
	)
}

// NotImplemented, but pan is to connect a runner to upstream services for gpg signing.
func AgentOptionGPGAgent(desc ...string) AgentOption {
	return AgentOptionNoop
}
