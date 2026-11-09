// Package egvm boots small Linux virtual machines, so Linux binaries can run where the host kernel
// is not Linux. A machine shares host directories with the guest over virtio-fs; the guest's init
// (vminit) mounts the one tagged root, runs a command chrooted into it, and powers the machine off.
package egvm

// Root is the tag of the shared directory vminit mounts as the root of the command it runs.
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
