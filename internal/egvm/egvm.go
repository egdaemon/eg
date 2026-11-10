// Package egvm boots small Linux virtual machines, so Linux binaries can run where the host kernel
// is not Linux. A machine's root is an egiso the host serves over NFS; the guest's init (vminit)
// mounts it and hands the machine to the root's own init, as a container runtime starts an image
// with /usr/sbin/init. The root's /etc/fstab mounts the host directories the machine shares over
// virtio-fs. vminit keeps running beside the root's init, forwarding NFS and running the commands
// the host sends it over vsock.
package egvm

import (
	"net"
	"path/filepath"
	"runtime"

	"github.com/egdaemon/eg/internal/userx"
)

// Share is a host directory the guest mounts over virtio-fs by its tag.
type Share struct {
	Tag string
	Dir string
}

// Machine describes a virtual machine: the Linux kernel and initramfs it boots, the kernel's
// command line, its size, its root, and the directories it shares.
type Machine struct {
	Kernel  string // an uncompressed arm64 Image
	Initrd  string
	Cmdline string // vminit execs the root's init= once the root is mounted
	CPUs    uint
	Memory  uint64                   // bytes
	Root    func(net.Listener) error // serves the machine's root over NFS on the listener until it closes
	Shares  []Share
}

// Directory holds the machine image machines boot: an uncompressed arm64 kernel, Image, and an
// initramfs holding vminit and the modules it loads, initramfs.
func Directory() string {
	return userx.DefaultConfigLocation("egvm.d")
}

// New describes a machine booting the image in dir, as large as the host, with root as its root
// and the root's systemd as its init. Container images hold no udev to announce the console, so
// systemd starts no getty on it: one would wait out its device's timeout before the machine boots.
func New(dir string, root func(net.Listener) error, shares ...Share) Machine {
	return Machine{
		Kernel:  filepath.Join(dir, "Image"),
		Initrd:  filepath.Join(dir, "initramfs"),
		Cmdline: "console=hvc0 quiet init=/usr/sbin/init systemd.getty_auto=0",
		CPUs:    uint(runtime.NumCPU()),
		Memory:  4 << 30,
		Root:    root,
		Shares:  shares,
	}
}
