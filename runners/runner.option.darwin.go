//go:build darwin

package runners

import (
	"log"

	"github.com/logrusorgru/aurora"
)

func AgentOptionHostOS() AgentOption {
	log.Println(aurora.NewAurora(true).Red("darwin is an experimental host, many features do not yet work."))
	return AgentOptionCompose(
		AgentOptionCommandLine("--userns", "host"), // properly map host user into containers.
		AgentOptionCommandLine("--privileged"),     // darwin permission issues.
	)
}
