//go:build !linux && !darwin

package runners

import (
	"errors"
	"log"

	"github.com/logrusorgru/aurora"
)

// Provide a noop option for unknown os
func AgentOptionHostOS(cli ...string) AgentOption {
	log.Println(aurora.NewAurora(true).Red("you're using an unknown host operating system, many things may not work correctly. feel free to report your findings to us for improvements."))
	return AgentOptionCommandLine(cli...) // escape hatch to allow customizing the container cli
}

func AgentOptionGPU(enabled bool) (AgentOption, error) {
	if !enabled {
		return AgentOptionNoop, nil
	}

	log.Println(aurora.NewAurora(true).Red("unknown hosts do not support gpu functionality. ignoring."))
	return AgentOptionNoop, errors.ErrUnsupported
}

// DetectGPU unknown hosts do not currently support gpu detection.
func DetectGPU() (driver string, vram uint64, err error) {
	return "", 0, nil
}
