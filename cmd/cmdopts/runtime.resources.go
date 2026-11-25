package cmdopts

import (
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/egdaemon/eg/runners"
)

type RuntimeResources struct {
	Arch   string        `flag:"" name:"arch" help:"native CPU architecture of the machine" default:"${vars_arch}"`
	OS     string        `flag:"" name:"os" help:"operating system of the machine" default:"${vars_os}"`
	Cores  uint64        `flag:"" name:"cores" help:"the number of vCPU to make available" default:"${vars_cores_minimum_default}"`
	Memory bytesx.Unit   `flag:"" name:"memory" help:"the amount of RAM to make available" default:"${vars_memory_minimum_default}"`
	Disk   bytesx.Unit   `flag:"" name:"disk" help:"the amount of disk space to make available" default:"${vars_disk_minimum_default}"`
	Vram   bytesx.Unit   `flag:"" name:"vram" help:"the amount of GPU memory to make available (unavailable, alpha, only in dev builds)" default:"${vars_vram_minimum_default}"`
	TTL    time.Duration `flag:"" name:"ttl" type:"durationinf" help:"maximum runtime for the upload. use 'infinity' to disable the ttl. infinite ttl is not supported for remote workloads." default:"1h"`
	// we lie about the limit here to encourage good end user behavior.
	Labels []string `flag:"" name:"label" help:"up to 10 labels to assign to this compute resource" default:"${vars_labels}"`
}

// Limits returns the capacity to schedule against: the configured resources capped at what the host has.
// a zero cores or memory is unconfigured and is the host's total. vram is never defaulted, zero is no vram.
func (t RuntimeResources) Limits(host runners.RuntimeResources) runners.RuntimeResources {
	return runners.RuntimeResources{
		Cores:  numericx.Min(langx.FirstNonZero(t.Cores, host.Cores), host.Cores),
		Memory: numericx.Min(langx.FirstNonZero(uint64(t.Memory), host.Memory), host.Memory),
		Vram:   numericx.Min(uint64(t.Vram), host.Vram),
	}
}

// KongVars returns the kong.Vars needed to satisfy RuntimeResources' own defaults
// (vars_arch, vars_os, vars_cores_minimum_default, vars_memory_minimum_default,
// vars_disk_minimum_default, vars_vram_minimum_default, vars_labels) as if t were the
// values cmd/eg/main.go derived from the host at startup. Primarily useful for building
// kong parsers in tests without duplicating those defaults by hand.
func (t RuntimeResources) KongVars() kong.Vars {
	return kong.Vars{
		"vars_arch":                   t.Arch,
		"vars_os":                     t.OS,
		"vars_cores_minimum_default":  strconv.FormatUint(t.Cores, 10),
		"vars_memory_minimum_default": t.Memory.String(),
		"vars_disk_minimum_default":   t.Disk.String(),
		"vars_vram_minimum_default":   t.Vram.String(),
		"vars_labels":                 strings.Join(t.Labels, ","),
	}
}
