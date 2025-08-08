package actl

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/dustin/go-humanize"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/egdaemon/eg/internal/stringsx"
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
	runtimecfg cmdopts.RuntimeResources
	AccountID  string `name:"account" help:"account to register runner with" default:"${vars_account_id}" required:"true"`
	Seed       string `name:"seed" help:"used to ensure a consistent secret is used, this is a sensitive value" placeholder:"00000000-0000-0000-0000-000000000000"`
}

func (t BootstrapEnvDaemon) Run(gctx *cmdopts.Global) (err error) {
	memory := numericx.Max(uint64(t.runtimecfg.Memory), uint64(float64(memory.TotalMemory())*0.9))

	return envx.Build().Var(
		"EG_ACCOUNT", t.AccountID,
	).Var(
		"EG_ENTROPY_SEED", stringsx.DefaultIfBlank(md5x.String(t.Seed), uuid.Must(uuid.NewV4()).String()),
	).Var(
		"EG_RESOURCES_CORES", strconv.FormatUint(numericx.Max(uint64(runtime.NumCPU()), t.runtimecfg.Cores), 10),
	).Var(
		"EG_RESOURCES_MEMORY", strconv.FormatUint(memory, 10),
	).Var(
		"EG_RESOURCES_DISK", humanize.Bytes(uint64(t.runtimecfg.Disk)),
	).CopyTo(os.Stdout)
}
