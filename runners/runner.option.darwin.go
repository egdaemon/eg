//go:build darwin

package runners

import (
	"fmt"
	"log"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/logrusorgru/aurora"
)

func AgentOptionHostOS() AgentOption {
	log.Println(aurora.NewAurora(true).Red("darwin is an experimental host, many features do not yet work."))
	return AgentOptionCompose(
		AgentOptionCommandLine("--userns", "host"), // properly map host user into containers.
		AgentOptionCommandLine("--privileged"),     // darwin permission issues.
	)
}

func AgentOptionContainerCache(dir string) string {
	if envx.Boolean(false, eg.EnvExperimentalContainerCache) {
		return AgentMountReadWrite(dir, "/var/lib/containers")
	}

	log.Println(aurora.NewAurora(true).Red(fmt.Sprintf("container cache is disabled on darwin %s=true to enable", eg.EnvExperimentalContainerCache)))
	return ""
}
