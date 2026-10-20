package actl

import (
	"fmt"
	"log"
	"runtime"
	"strconv"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
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
	memory := bytesx.Unit(numericx.Min(langx.FirstNonZero(uint64(t.RuntimeResources.Memory), available), available))

	_, gpuvram, err := runners.DetectGPU()
	if err != nil {
		log.Println("unable to detect gpu:", err)
	}

	vram := bytesx.Unit(numericx.Min(langx.FirstNonZero(uint64(t.RuntimeResources.Vram), gpuvram), gpuvram))

	environ := envx.Build().Var(
		"EG_RUNNER_CPU", strconv.FormatUint(numericx.Min(langx.FirstNonZero(t.RuntimeResources.Cores, uint64(runtime.NumCPU())), uint64(runtime.NumCPU())), 10),
	).Var(
		"EG_RUNNER_MEMORY", fmt.Sprintf("%v", memory),
	).Var(
		"EG_RUNNER_VRAM", fmt.Sprintf("%v", vram),
	)

	// the runner units default to the egnet bridge when this is unset.
	if t.LAN {
		environ.Var("EG_RUNNER_NETWORK", "egnet-lan")
	}

	return environ.CopyTo(kctx.Stdout)
}
