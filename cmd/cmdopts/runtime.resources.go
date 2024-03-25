package cmdopts

type RuntimeResources struct {
	Arch   string `help:"native CPU architecture of the machine" default:"${vars_arch}"`
	OS     string `help:"operating system of the machine" default:"${vars_os}"`
	Cores  uint64 `help:"the number of vCPU to make available" default:"${vars_cores_minimum_default}"`
	Memory uint64 `help:"the amount of RAM to make available" default:"${vars_memory_minimum_default}"`
	Disk   uint64 `help:"the amount of disk space to make available" default:"${vars_disk_minimum_default}"`
	Vram   uint64 `help:"the amount of GPU memory to make available" default:"0"`
}
