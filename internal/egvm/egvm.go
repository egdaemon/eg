// Package egvm boots small Linux virtual machines, so Linux binaries can run where the host kernel
// is not Linux. A machine's root is an egiso the host serves over NFS; the guest's init (vminit)
// mounts it and hands the machine to the root's own init, as a container runtime starts an image
// with /usr/sbin/init. The kernel's command line tells the root's systemd what the machine attaches:
// the virtio-fs shares to mount and the network to configure. vminit keeps running beside the root's
// init, forwarding NFS and running the commands the host sends it over vsock.
package egvm

import (
	"encoding/base64"
	"fmt"
	"runtime"
	"strings"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/userx"
)

// Share is a host directory the machine mounts over virtio-fs at Path.
type Share struct {
	Tag     string
	Dir     string
	Path    string
	Options string
}

// Mount is a mount of the machine's own, which systemd orders after the shares it lies under.
type Mount struct {
	What    string
	Where   string
	Type    string
	Options string
}

// Machine describes a virtual machine: the image it boots, its size, its root, and what it mounts.
type Machine struct {
	dir    string
	init   string
	cpus   uint
	memory uint64
	root   *egiso.SysFS
	nfs    []egiso.NFSOption
	shares []Share
	mounts []Mount
}

// Option changes the machine New describes.
type Option func(*Machine)

// Directory holds the machine image machines boot: an uncompressed arm64 kernel, Image, and an
// initramfs holding vminit and the modules it loads, initramfs.
func Directory() string {
	return userx.DefaultConfigLocation("egvm.d")
}

// New describes a machine rooted at root that boots the image in Directory, as large as the host,
// with the root's systemd as its init.
func New(root *egiso.SysFS, options ...Option) Machine {
	return langx.Clone(Machine{
		dir:    Directory(),
		init:   "/usr/sbin/init",
		cpus:   uint(runtime.NumCPU()),
		memory: 4 << 30,
		root:   root,
	}, options...)
}

// OptionDirectory boots the machine image in dir.
func OptionDirectory(dir string) Option {
	return func(m *Machine) {
		m.dir = dir
	}
}

// OptionInit runs path as the machine's pid 1.
func OptionInit(path string) Option {
	return func(m *Machine) {
		m.init = path
	}
}

// OptionAccounts reports the root's owners to the machine as these accounts' ids.
func OptionAccounts(users []egiso.Account, groups []egiso.Account) Option {
	return func(m *Machine) {
		m.nfs = append(m.nfs, egiso.NFSOptionAccounts(users, groups))
	}
}

// OptionShare attaches the share's directory to the machine and mounts it.
func OptionShare(s Share) Option {
	return func(m *Machine) {
		m.shares = append(m.shares, s)
	}
}

// OptionMount mounts what at where once the machine's shares are mounted.
func OptionMount(mnt Mount) Option {
	return func(m *Machine) {
		m.mounts = append(m.mounts, mnt)
	}
}

// cmdline is the machine's kernel command line. Container images hold no udev to announce the
// console, so systemd starts no getty on it: one would wait out its device's timeout before the
// machine boots. systemd-fstab-generator mounts the shares and mounts from systemd.mount-extra, and
// systemd-network-generator configures the machine's NAT device for networkd from ip=. The
// tmpfiles.extra credential points resolv.conf at resolved, replacing the empty placeholder image
// builds leave behind.
func (t Machine) cmdline() string {
	var (
		resolv = base64.StdEncoding.EncodeToString([]byte("L+ /etc/resolv.conf - - - - ../run/systemd/resolve/stub-resolv.conf\n"))
		params = []string{
			"console=hvc0",
			"quiet",
			"init=" + t.init,
			"systemd.getty_auto=0",
			"ip=eth0:dhcp",
			"systemd.wants=systemd-network-generator.service",
			"systemd.wants=systemd-networkd.service",
			"systemd.set_credential_binary=tmpfiles.extra:" + resolv,
		}
	)

	for _, s := range t.shares {
		params = append(params, fmt.Sprintf("systemd.mount-extra=%s:%s:virtiofs:%s", s.Tag, s.Path, s.Options))
	}
	for _, m := range t.mounts {
		params = append(params, fmt.Sprintf("systemd.mount-extra=%s:%s:%s:%s", m.What, m.Where, m.Type, m.Options))
	}

	return strings.Join(params, " ")
}
