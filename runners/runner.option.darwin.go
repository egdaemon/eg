//go:build darwin

package runners

import (
	"log"

	"github.com/logrusorgru/aurora"
)

func AgentOptionHostOS() AgentOption {
	log.Println(aurora.NewAurora(true).Red("darwin is currently an alpha host, you may experience some functionality issues, please report your findings to us for support/fixes."))
	return AgentOptionCompose(
		AgentOptionCommandLine("--userns", "host"),   // properly map host user into containers.
		AgentOptionCommandLine("--privileged"),       // darwin permission issues.
		AgentOptionCommandLine("--pids-limit", "-1"), // ensure we dont run into pid limits.
	)
}
