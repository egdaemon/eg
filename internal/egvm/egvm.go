// Package egvm boots small Linux virtual machines, so Linux binaries can run where the host kernel
// is not Linux. A machine shares host directories with the guest over virtio-fs; the guest's init
// (vminit) mounts the one tagged root and runs the commands the host sends it over vsock chrooted
// into it.
package egvm

import (
	"path/filepath"
	"runtime"

	"github.com/egdaemon/eg/internal/userx"
)

// Root is the tag of the shared directory vminit mounts as the root of the commands it runs.
const Root = "root"

// Share is a host directory the guest mounts over virtio-fs by its tag.
type Share struct {
	Tag string
	Dir string
}

// Machine describes a virtual machine: the Linux kernel and initramfs it boots, the kernel's
// command line, its size, and the directories it shares.
type Machine struct {
	Kernel  string // an uncompressed arm64 Image
	Initrd  string
	Cmdline string
	CPUs    uint
	Memory  uint64 // bytes
	Shares  []Share
}

// Directory holds the machine image cells boot: an uncompressed arm64 kernel, Image, and an
// initramfs holding vminit and the modules it loads, initramfs.
func Directory() string {
	return userx.DefaultConfigLocation("egvm.d")
}

// New describes a machine booting the image in dir, as large as the host, with root as the root of
// its commands.
func New(dir string, root string) Machine {
	return Machine{
		Kernel:  filepath.Join(dir, "Image"),
		Initrd:  filepath.Join(dir, "initramfs"),
		Cmdline: "console=hvc0 quiet",
		CPUs:    uint(runtime.NumCPU()),
		Memory:  4 << 30,
		Shares:  []Share{{Tag: Root, Dir: root}},
	}
}
