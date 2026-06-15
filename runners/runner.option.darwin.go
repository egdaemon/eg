//go:build darwin

package runners

import (
	"errors"
	"log"

	"github.com/logrusorgru/aurora"
)

func AgentOptionHostOS(cli ...string) AgentOption {
	log.Println(aurora.NewAurora(true).Red("darwin is currently an alpha host, you may experience some functionality issues, please report your findings to us for support/fixes."))
	return AgentOptionCompose(
		AgentOptionCommandLine("--userns", "host"),   // properly map host user into containers.
		AgentOptionCommandLine("--privileged"),       // darwin permission issues.
		AgentOptionCommandLine("--pids-limit", "-1"), // ensure we dont run into pid limits.
		AgentOptionCommandLine(cli...),               // escape hatch to allow customizing the container cli
	)
}

func AgentOptionGPU(enabled bool) (AgentOption, error) {
	if !enabled {
		return AgentOptionNoop, nil
	}

	log.Println(aurora.NewAurora(true).Red("darwin currently doesnt support gpu functionality. ignoring."))
	return AgentOptionNoop, errors.ErrUnsupported
}

// DetectGPU darwin does not currently support gpu detection.
func DetectGPU() (driver string, vram uint64, err error) {
	return "", 0, nil
}
