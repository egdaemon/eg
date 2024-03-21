package wasix

import (
	"strings"

	"github.com/tetratelabs/wazero"
)

func Environ(mcfg wazero.ModuleConfig, environ ...string) wazero.ModuleConfig {
	for _, v := range environ {
		if k, v, ok := strings.Cut(v, "="); ok {
			mcfg = mcfg.WithEnv(k, v)
		}
	}

	return mcfg
}
