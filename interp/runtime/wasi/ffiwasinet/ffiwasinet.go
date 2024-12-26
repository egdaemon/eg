package ffiwasinet

import (
	"github.com/egdaemon/wasinet/wasinet/wnetruntime"
	"github.com/egdaemon/wasinet/wazeronet"
	"github.com/tetratelabs/wazero"
)

func Wazero(runtime wazero.Runtime) wazero.HostModuleBuilder {
	return wazeronet.Module(runtime, wnetruntime.Unrestricted())
}
