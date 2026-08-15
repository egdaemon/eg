package cmdopts

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/internal/bytesx"
)

type RuntimeResources struct {
	Arch   string        `flag:"" name:"arch" help:"native CPU architecture of the machine" default:"${vars_arch}"`
	OS     string        `flag:"" name:"os" help:"operating system of the machine" default:"${vars_os}"`
	Cores  uint64        `flag:"" name:"cores" help:"the number of vCPU to make available" default:"${vars_cores_minimum_default}"`
	Memory bytesx.Unit   `flag:"" name:"memory" help:"the amount of RAM to make available" default:"${vars_memory_minimum_default}"`
	Disk   bytesx.Unit   `flag:"" name:"disk" help:"the amount of disk space to make available" default:"${vars_disk_minimum_default}"`
	Vram   bytesx.Unit   `flag:"" name:"vram" help:"the amount of GPU memory to make available (unavailable, alpha, only in dev builds)" default:"${vars_vram_minimum_default}"`
	TTL    time.Duration `flag:"" name:"ttl" type:"durationinf" help:"maximum runtime for the upload. use 'infinity' to disable the ttl. infinite ttl is not supported for remote workloads." default:"1h"`
	Labels []string      `flag:"" name:"label" help:"up to 10 labels to assign to this compute resource" default:"${vars_labels}"`
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
		"vars_memory_minimum_default": fmt.Sprintf("%v", t.Memory),
		"vars_disk_minimum_default":   fmt.Sprintf("%v", t.Disk),
		"vars_vram_minimum_default":   fmt.Sprintf("%v", t.Vram),
		"vars_labels":                 strings.Join(t.Labels, ","),
	}
}
