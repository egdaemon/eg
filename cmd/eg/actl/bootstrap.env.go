package actl

import (
	"encoding/base64"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/cryptox"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/gofrs/uuid/v5"
	"github.com/pbnjay/memory"
)

type BootstrapEnv struct {
	Runner BootstrapEnvRunner `cmd:"" help:"bootstrap the a runner service environment file"`
	Daemon BootstrapEnvDaemon `cmd:"" help:"bootstrap the a daemon service environment file"`
}

type BootstrapEnvRunner struct {
	runtimecfg cmdopts.RuntimeResources
}

func (t BootstrapEnvRunner) Run(gctx *cmdopts.Global) (err error) {
	memory := bytesx.Unit(numericx.Max(uint64(t.runtimecfg.Memory), uint64(float64(memory.TotalMemory())*0.9)))

	return envx.Build().Var(
		"EG_RUNNER_CPU", strconv.FormatUint(numericx.Max(uint64(runtime.NumCPU()), t.runtimecfg.Cores), 10),
	).Var(
		"EG_RUNNER_MEMORY", fmt.Sprintf("%v", memory),
	).CopyTo(os.Stdout)
}

type BootstrapEnvDaemon struct {
	cmdopts.RuntimeResources
	AccountID string `name:"account" help:"account to register runner with" default:"${vars_account_id}" required:"true"`
	Seed      string `name:"seed" help:"used to ensure a consistent secret is used, this is a sensitive value" placeholder:"00000000-0000-0000-0000-000000000000"`
	Workers   uint64 `name:"workers" help:"specify the maximum concurrent workload capacity"`
}

func (t BootstrapEnvDaemon) Run(gctx *cmdopts.Global) (err error) {
	memory := numericx.Max(uint64(t.Memory), uint64(float64(memory.TotalMemory())*0.9))
	prng := cryptox.NewChaCha8(langx.FirstNonZero(t.Seed, uuid.Must(uuid.NewV4()).String()))

	seed, err := io.ReadAll(io.LimitReader(prng, rand.New(prng).Int64N(128)))
	if err != nil {
		return errorsx.Wrap(err, "failed to generate entropy")
	}

	environ := envx.Build().Var(
		"EG_ACCOUNT", t.AccountID,
	).Var(
		"EG_ENTROPY_SEED", base64.RawURLEncoding.EncodeToString(seed),
	).Var(
		"EG_RESOURCES_CORES", strconv.FormatUint(numericx.Max(uint64(runtime.NumCPU()), t.Cores), 10),
	).Var(
		"EG_RESOURCES_MEMORY", strconv.FormatUint(memory, 10),
	).Var(
		"EG_RESOURCES_DISK", fmt.Sprintf("\"%s\"", string(errorsx.Zero(t.Disk.MarshalText()))),
	)

	if t.Workers > 0 {
		environ.Var(
			"EG_COMPUTE_WORKLOAD_CAPACITY", strconv.FormatUint(t.Workers, 10),
		)
	}

	if len(t.Labels) > 0 {
		environ.Var(
			"EG_LABELS", fmt.Sprintf("\"%s\"", strings.Join(t.Labels, ",")),
		)
	}

	return environ.CopyTo(os.Stdout)
}
