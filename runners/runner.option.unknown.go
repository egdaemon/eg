//go:build !linux && !darwin

package runners

import (
	"log"

	"github.com/logrusorgru/aurora"
)

// Provide a noop option for unknown os
func AgentOptionHostOS() AgentOption {
	log.Println(aurora.NewAurora(true).Red("you're using an unknown host operating system, many things may not work correctly"))
	return AgentOptionNoop
}
