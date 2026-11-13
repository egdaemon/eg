package runners

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/userx"
)

// configures a local machine run to connect to the users gpg agent.
func AgentOptionLocalGPGAgent(ctx context.Context, envb *envx.Builder) AgentOption {
	agentsock, gnupghome, err := localgpg()
	if err != nil {
		log.Println("unable to check if gpg config is available", err)
		return AgentOptionNoop
	}

	return agentOptionLocalGPGAgent(ctx, envb, agentsock, gnupghome)
}

func agentOptionLocalGPGAgent(ctx context.Context, envb *envx.Builder, agentsock string, gnupghome string) AgentOption {
	if !gpgavailable(agentsock, gnupghome) {
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

// MachineOptionLocalGPGAgent configures a local machine run to connect to the users gpg agent, as
// AgentOptionLocalGPGAgent configures a container.
func MachineOptionLocalGPGAgent(envb *envx.Builder) MachineOption {
	agentsock, gnupghome, err := localgpg()
	if err != nil {
		log.Println("unable to check if gpg config is available", err)
		return MachineOptionNoop
	}

	return machineOptionLocalGPGAgent(envb, agentsock, gnupghome)
}

func machineOptionLocalGPGAgent(envb *envx.Builder, agentsock string, gnupghome string) MachineOption {
	if !gpgavailable(agentsock, gnupghome) {
		return MachineOptionNoop
	}

	envb.Var("GNUPGHOME", eg.DefaultMountRoot(".gnupg"))

	runtimedir := filepath.Dir(agentsock)
	return langx.Compose(
		MachineOptionOverlay(gnupghome, eg.DefaultMountRoot(".gnupg")),
		MachineOptionSocket(agentsock, eg.DefaultMountRoot(".gnupg", "S.gpg-agent")),
		MachineOptionSocket(filepath.Join(runtimedir, "S.dirmngr"), eg.DefaultMountRoot(".gnupg", "S.dirmngr")),
		MachineOptionSocket(filepath.Join(runtimedir, "S.keyboxd"), eg.DefaultMountRoot(".gnupg", "S.keyboxd")),
	)
}

// localgpg locates the user's gpg agent socket and gpg home.
func localgpg() (agentsock string, gnupghome string, err error) {
	// https://wiki.gnupg.org/AgentForwarding
	// https://github.com/gpg/gnupg/blob/master/tools/gpg-connect-agent.c
	// https://blog.packagecloud.io/how-to-gpg-sign-and-verify-deb-packages-and-apt-repositories/
	if gnupghome, err = userx.HomeDirectory(".gnupg"); err != nil {
		return "", "", err
	}

	return userx.DefaultRuntimeDirectory("gnupg", "S.gpg-agent"), gnupghome, nil
}

// gpgavailable reports whether the gpg agent socket and gpg home exist.
func gpgavailable(agentsock string, gnupghome string) bool {
	if _, err := os.Stat(agentsock); fsx.ErrIsNotExist(err) != nil {
		log.Println("gpg agent is not available", err)
		return false
	} else if err != nil {
		log.Println("unable to check if gpg agent is available", err)
		return false
	}

	if _, err := os.Stat(gnupghome); fsx.ErrIsNotExist(err) != nil {
		log.Println("gpg config is not available", err)
		return false
	} else if err != nil {
		log.Println("unable to check if gpg config is available", err)
		return false
	}

	return true
}

// NotImplemented, but plan is to connect a runner to upstream services for gpg signing.
func AgentOptionGPGAgent(desc ...string) AgentOption {
	return AgentOptionNoop
}
