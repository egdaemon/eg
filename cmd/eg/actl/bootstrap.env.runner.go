package actl

import (
	"log"
	"runtime"

	"github.com/alecthomas/kong"
	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/netx"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/egdaemon/eg/runners"
	"github.com/pbnjay/memory"
)

type BootstrapEnvRunner struct {
	cmdopts.RuntimeResources
	LAN bool `name:"lan" help:"run the runner container on the local network (the egnet-lan network) instead of the private bridge so it can be found by clients on the same network, see 'eg compute lan'" negatable:""`
}

func (t BootstrapEnvRunner) Run(kctx *kong.Context, gctx *cmdopts.Global) (err error) {
	available := uint64(float64(memory.TotalMemory()) * 0.9)
	memory := numericx.Min(langx.FirstNonZero(t.RuntimeResources.Memory, bytesx.Unit(available)), bytesx.Unit(available))

	_, gpuvram, err := runners.DetectGPU()
	if err != nil {
		log.Println("unable to detect gpu:", err)
	}

	vram := numericx.Min(langx.FirstNonZero(t.RuntimeResources.Vram, gpuvram), gpuvram)

	environ := envx.Build()
	environ.Var(_eg.EnvRunnerCPU, numericx.Min(langx.FirstNonZero(t.RuntimeResources.Cores, uint64(runtime.NumCPU())), uint64(runtime.NumCPU())), envx.FormatOmitZero)
	environ.Var(_eg.EnvRunnerMemory, memory, envx.FormatOmitZero, envx.FormatQuote)
	environ.Var(_eg.EnvRunnerVRAM, vram, envx.FormatOmitZero, envx.FormatQuote)

	// the interface the egnet-lan network is attached to, see eg-runner-build.service.
	if iface, err := netx.DefaultRouteInterface(); err != nil {
		log.Println("unable to detect the default network interface:", err)
	} else {
		environ.Var(_eg.EnvRunnerDefaultNetworkInterface, iface, envx.FormatOmitZero, envx.FormatQuote)
	}

	// the runner units default to the egnet bridge when this is unset.
	if t.LAN {
		environ.Var(_eg.EnvRunnerNetwork, "egnet-lan")
	}

	return environ.CopyTo(kctx.Stdout)
}
