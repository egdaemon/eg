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
	AccountID        string        `name:"account" help:"account to register runner with" default:"${vars_account_id}" env:"EG_ACCOUNT" required:"true"`
	Seed             string        `name:"seed" help:"used to ensure a consistent secret is used, this is a sensitive value" default:"${vars_entropy_seed}" placeholder:"00000000-0000-0000-0000-000000000000"`
	Workers          uint64        `name:"workers" help:"specify the maximum concurrent workload capacity" default:"${vars_cores_minimum_default}"`
	Compilers        uint64        `name:"compilers" help:"specify the maximum number of workloads compiled concurrently, the daemon's default applies when unset"`
	TargetLoad       float64       `name:"target-load" help:"cpu load to target before the scheduler stops pulling new workloads, the daemon's default applies when unset"`
	API              bool          `name:"api" help:"accept workloads pushed directly to this runner (POST /c/upload, POST /c/enqueue) and advertise it to the control plane" default:"true" negatable:""`
	RestartOnUpgrade string        `name:"restart-on-upgrade" help:"restart the daemon when the eg package is upgraded: auto, yes or no" enum:"auto,yes,no" default:"auto"`
	MDNSAdvertise    bool          `name:"mdns-advertise" help:"advertise this daemon on the local network (multicast dns) so clients on the same network can upload to it directly" negatable:""`
	LANPeerTTL       time.Duration `name:"lan-peer-ttl" type:"durationinf" help:"how long systems on the local network are remembered after they were last seen, the daemon's default applies when unset"`
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

	environ.Var(_eg.EnvRestartOnUpgrade, t.RestartOnUpgrade)

	if t.Workers > 0 {
		environ.Var(
			_eg.EnvComputeWorkloadCapacity, strconv.FormatUint(t.Workers, 10),
		)
	}

	if t.Compilers > 0 {
		environ.Var(
			_eg.EnvComputeCompileCapacity, strconv.FormatUint(t.Compilers, 10),
		)
	}

	if t.TargetLoad > 0 {
		environ.Var(
			_eg.EnvComputeWorkloadTargetLoad, strconv.FormatFloat(t.TargetLoad, 'f', -1, 64),
		)
	}

	// the daemon accepts pushed workloads by default, only emit when it is disabled.
	if !t.API {
		environ.Var(_eg.EnvComputeAPIEnabled, "false")
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
