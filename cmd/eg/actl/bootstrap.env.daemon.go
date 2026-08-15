package actl

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/egdaemon/eg/runners"
	"github.com/pbnjay/memory"
)

type BootstrapEnvDaemon struct {
	cmdopts.RuntimeResources
	AccountID string `name:"account" help:"account to register runner with" default:"${vars_account_id}" env:"EG_ACCOUNT" required:"true"`
	Seed      string `name:"seed" help:"used to ensure a consistent secret is used, this is a sensitive value" default:"${vars_entropy_seed}" placeholder:"00000000-0000-0000-0000-000000000000"`
	Workers   uint64 `name:"workers" help:"specify the maximum concurrent workload capacity"`
}

func (t BootstrapEnvDaemon) Run(kctx *kong.Context, gctx *cmdopts.Global, entropy cmdopts.Entropy) (err error) {
	memory := numericx.Max(uint64(t.Memory), uint64(float64(memory.TotalMemory())*0.9))

	seed, err := entropy(t.Seed)
	if err != nil {
		return errorsx.Wrap(err, "failed to generate entropy")
	}

	gpudriver, gpuvram, err := runners.DetectGPU()
	if err != nil {
		log.Println("unable to detect gpu:", err)
	}

	labels := t.Labels
	if gpudriver != "" {
		labels = append(labels, fmt.Sprintf("eg:gpu:%s", gpudriver))
	}

	environ := envx.Build().Var(
		"EG_ACCOUNT", t.AccountID,
	).Var(
		"EG_ENTROPY_SEED", seed,
	).Var(
		"EG_RESOURCES_CORES", strconv.FormatUint(numericx.Max(uint64(runtime.NumCPU()), t.Cores), 10),
	).Var(
		"EG_RESOURCES_MEMORY", strconv.FormatUint(memory, 10),
	).Var(
		"EG_RESOURCES_DISK", fmt.Sprintf("\"%s\"", string(errorsx.Zero(t.Disk.MarshalText()))),
	).Var(
		"EG_RESOURCES_VRAM", strconv.FormatUint(numericx.Max(uint64(t.Vram), gpuvram), 10),
	)

	if t.Workers > 0 {
		environ.Var(
			_eg.EnvComputeWorkloadCapacity, strconv.FormatUint(t.Workers, 10),
		)
	}

	if len(labels) > 0 {
		environ.Var(
			"EG_LABELS", fmt.Sprintf("\"%s\"", strings.Join(labels, ",")),
		)
	}

	return environ.CopyTo(kctx.Stdout)
}
