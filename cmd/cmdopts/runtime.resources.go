package cmdopts

import (
	"time"

	"github.com/egdaemon/eg/internal/bytesx"
)

type RuntimeResources struct {
	Arch   string        `name:"arch" help:"native CPU architecture of the machine" default:"${vars_arch}"`
	OS     string        `name:"os" help:"operating system of the machine" default:"${vars_os}"`
	Cores  uint64        `name:"cores" help:"the number of vCPU to make available" default:"${vars_cores_minimum_default}"`
	Memory bytesx.Unit   `name:"memory" help:"the amount of RAM to make available" default:"${vars_memory_minimum_default}"`
	Disk   bytesx.Unit   `name:"disk" help:"the amount of disk space to make available" default:"${vars_disk_minimum_default}"`
	Vram   bytesx.Unit   `name:"vram" help:"the amount of GPU memory to make available (unavailable, alpha, only in dev builds)" default:"${vars_vram_minimum_default}"`
	TTL    time.Duration `name:"ttl" help:"maximum runtime for the upload" default:"1h"`
	Labels []string      `name:"label" help:"up to 10 labels to assign to this compute resource"`
}
