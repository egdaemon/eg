package actl

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/egdaemon/eg/runners"
)

type BootstrapEnvDaemon struct {
	cmdopts.RuntimeResources
	AccountID string `name:"account" help:"account to register runner with" default:"${vars_account_id}" env:"EG_ACCOUNT" required:"true"`
	Seed      string `name:"seed" help:"used to ensure a consistent secret is used, this is a sensitive value" default:"${vars_entropy_seed}" placeholder:"00000000-0000-0000-0000-000000000000"`
	Workers   uint64 `name:"workers" help:"specify the maximum concurrent workload capacity"`

	MDNSAdvertise bool          `name:"mdns-advertise" help:"advertise this daemon on the local network (multicast dns) so clients on the same network can upload to it directly" negatable:""`
	LANPeerTTL    time.Duration `name:"lan-peer-ttl" type:"durationinf" help:"how long systems on the local network are remembered after they were last seen, the daemon's default applies when unset"`
}

func (t BootstrapEnvDaemon) Run(kctx *kong.Context, gctx *cmdopts.Global, entropy cmdopts.Entropy) (err error) {
	seed, err := entropy(t.Seed)
	if err != nil {
		return errorsx.Wrap(err, "failed to generate entropy")
	}

	gpudriver, gpuvram, err := runners.DetectGPU()
	if err != nil {
		log.Println("unable to detect gpu:", err)
	}

	labels := t.RuntimeResources.Labels
	if gpudriver != "" {
		labels = append(labels, fmt.Sprintf("eg:gpu:%s", gpudriver))
	}

	environ := envx.Build().Var(
		"EG_ACCOUNT", t.AccountID,
	).Var(
		"EG_ENTROPY_SEED", seed,
	).Var(
		"EG_RESOURCES_CORES", strconv.FormatUint(numericx.Max(uint64(runtime.NumCPU()), t.RuntimeResources.Cores), 10),
	).Var(
		"EG_RESOURCES_MEMORY", strconv.FormatUint(uint64(t.RuntimeResources.Memory), 10),
	).Var(
		"EG_RESOURCES_DISK", fmt.Sprintf("\"%s\"", string(errorsx.Zero(t.RuntimeResources.Disk.MarshalText()))),
	).Var(
		"EG_RESOURCES_VRAM", strconv.FormatUint(numericx.Max(uint64(t.RuntimeResources.Vram), gpuvram), 10),
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

	if t.MDNSAdvertise {
		environ.Var("EG_MDNS_ADVERTISE", "true")
	}

	if t.LANPeerTTL > 0 {
		environ.Var("EG_LAN_PEER_TTL", fmt.Sprintf("\"%s\"", t.LANPeerTTL))
	}

	return environ.CopyTo(kctx.Stdout)
}
