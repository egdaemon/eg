package actl

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg"
	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/egdaemon/eg/runners"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

type BootstrapEnvDaemon struct {
	cmdopts.RuntimeResources
	AccountID        string        `name:"account" help:"account to register runner with" default:"${vars_account_id}" env:"${env_eg_account}" required:"true"`
	Seed             string        `name:"seed" help:"used to ensure a consistent secret is used, this is a sensitive value" default:"${vars_entropy_seed}" placeholder:"00000000-0000-0000-0000-000000000000"`
	Workers          uint64        `name:"workers" help:"specify the maximum concurrent workload capacity" default:"${vars_cores_minimum_default}"`
	Compilers        uint64        `name:"compilers" help:"specify the maximum number of workloads compiled concurrently, the daemon's default applies when unset"`
	TargetLoad       float64       `name:"target-load" help:"cpu load to target before the scheduler stops pulling new workloads, the daemon's default applies when unset"`
	API              bool          `name:"api" help:"accept workloads pushed directly to this runner (POST /c/upload, POST /c/enqueue) and advertise it to the control plane" default:"true" negatable:""`
	RestartOnUpgrade string        `name:"restart-on-upgrade" help:"restart the daemon when the eg package is upgraded: auto, yes or no" enum:"auto,yes,no" default:"auto"`
	MDNSAdvertise    bool          `name:"mdns-advertise" help:"advertise this daemon on the local network (multicast dns) so clients on the same network can upload to it directly" negatable:""`
	VCSCache         bool          `name:"vcs-cache" help:"experimental: mirror cloned repositories in the workload cache so repeat clones are incremental" negatable:""`
	LANPeerTTL       time.Duration `name:"lan-peer-ttl" type:"durationinf" help:"how long systems on the local network are remembered after they were last seen, the daemon's default applies when unset"`
	Environment      []string      `name:"env" short:"e" help:"define environment variables and their values to be included"`
	EnvironmentPaths []string      `name:"envpath" help:"environment files to include" default:""`
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

	// requested resources are capped at what the machine physically has.
	physmemory, physdisk := uint64(math.MaxUint64), uint64(math.MaxUint64)
	if vm, err := mem.VirtualMemory(); err != nil {
		log.Println("unable to detect memory:", err)
	} else {
		physmemory = vm.Total
	}

	if du, err := disk.Usage("/"); err != nil {
		log.Println("unable to detect disk:", err)
	} else {
		physdisk = du.Total
	}

	environ := envx.Build()
	environ.Var(eg.EnvAccount, t.AccountID)
	environ.Var(eg.EnvEGEntropySeed, seed)
	environ.Var(_eg.EnvRestartOnUpgrade, t.RestartOnUpgrade)
	environ.Var(_eg.EnvComputeResourceCores, numericx.Min(t.RuntimeResources.Cores, uint64(runtime.NumCPU())))
	environ.Var(_eg.EnvComputeResourceMemory, numericx.Min(t.RuntimeResources.Memory, bytesx.Unit(physmemory)), envx.FormatQuote)
	environ.Var(_eg.EnvComputeResourceDisk, numericx.Min(t.RuntimeResources.Disk, bytesx.Unit(physdisk)), envx.FormatQuote)
	environ.Var(_eg.EnvComputeResourceVram, numericx.Min(langx.FirstNonZero(t.RuntimeResources.Vram, gpuvram), gpuvram), envx.FormatQuote)
	environ.Var(_eg.EnvComputeWorkloadCapacity, t.Workers, envx.FormatOmitZero)
	environ.Var(_eg.EnvComputeCompileCapacity, t.Compilers, envx.FormatOmitZero)
	environ.Var(_eg.EnvComputeWorkloadTargetLoad, t.TargetLoad, envx.FormatOmitZero)
	environ.Var(_eg.EnvComputeAPIEnabled, t.API)
	environ.Var(_eg.EnvComputeResourceLabels, envx.SliceFormat(",", labels...), envx.FormatQuote)
	environ.Var(_eg.EnvComputeVCSCache, t.VCSCache)
	environ.Var(_eg.EnvLANMDNSAdvertise, t.MDNSAdvertise)
	environ.Var(_eg.EnvLANPeerTTL, t.LANPeerTTL, envx.FormatOmitZero)

	environ.FromPath(t.EnvironmentPaths...).FromEnviron(envx.AutoEnviron(t.Environment...)...)

	return environ.CopyTo(kctx.Stdout)
}
