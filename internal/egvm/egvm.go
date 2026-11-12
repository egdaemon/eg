// Package egvm boots small Linux virtual machines, so Linux binaries can run where the host kernel
// is not Linux. A machine's root is an egiso the host serves over NFS; the guest's init (vminit)
// mounts it and hands the machine to the root's own init, as a container runtime starts an image
// with /usr/sbin/init. The kernel's command line tells the root's systemd what the machine attaches:
// the virtio-fs devices to mount and the network to configure. vminit keeps running beside the root's
// init, forwarding NFS. On Linux the machine shares the host's kernel: the root's init runs in
// namespaces of its own, rooted at the egiso served over FUSE. Commands run over ssh into the root's
// own sshd, which systemd-ssh-generator binds to vsock port 22 in a virtual machine and to
// /run/ssh-unix-local/socket on Linux, as root with a key the machine authorizes when it boots.
package egvm

import (
	"encoding/base64"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/userx"
)

// resolvlink is the tmpfiles.extra credential pointing resolv.conf at resolved, replacing the empty
// placeholder image builds leave behind.
const resolvlink = "L+ /etc/resolv.conf - - - - ../run/systemd/resolve/stub-resolv.conf\n"

// mount is a host directory the machine attaches as the virtio-fs device tag and mounts at path.
type mount struct {
	tag      string
	dir      string
	path     string
	readonly bool
}

// Machine describes a virtual machine: the image it boots, its size, its root, and what it mounts.
// A memory of 0 is the host's.
type Machine struct {
	dir    string
	init   string
	cpus   uint
	memory uint64
	root   *egiso.SysFS
	nfs    []egiso.NFSOption
	mounts []mount
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
		dir:  Directory(),
		init: "/usr/sbin/init",
		cpus: uint(runtime.NumCPU()),
		root: root,
	}, options...)
}

// OptionCPUs gives the machine n cpus; 0 keeps the host's.
func OptionCPUs(n uint) Option {
	return func(m *Machine) {
		m.cpus = langx.FirstNonZero(n, m.cpus)
	}
}

// OptionMemory gives the machine n bytes of memory; 0 keeps the host's.
func OptionMemory(n uint64) Option {
	return func(m *Machine) {
		m.memory = n
	}
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

// OptionMountReadWrite mounts the host directory dir in the machine at path.
func OptionMountReadWrite(dir string, path string) Option {
	return optionMount(dir, path, false)
}

// OptionMountReadOnly mounts the host directory dir in the machine at path, read only.
func OptionMountReadOnly(dir string, path string) Option {
	return optionMount(dir, path, true)
}

func optionMount(dir string, path string, readonly bool) Option {
	return func(m *Machine) {
		m.mounts = append(m.mounts, mount{tag: fmt.Sprintf("egvm%d", len(m.mounts)), dir: dir, path: path, readonly: readonly})
	}
}

// ExitError is the exit code of a command that exited with other than 0.
type ExitError struct {
	Code int32
}

func (t ExitError) Error() string {
	return fmt.Sprintf("exit code %d", t.Code)
}

// stdio is a command's standard streams.
type stdio struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// ExecOption changes the stdio of a command Exec runs.
type ExecOption func(*stdio)

// ExecOptionStdin feeds r to the command until r ends.
func ExecOptionStdin(r io.Reader) ExecOption {
	return func(s *stdio) {
		s.stdin = r
	}
}

// ExecOptionStdout copies the command's stdout to w.
func ExecOptionStdout(w io.Writer) ExecOption {
	return func(s *stdio) {
		s.stdout = w
	}
}

// ExecOptionStderr copies the command's stderr to w.
func ExecOptionStderr(w io.Writer) ExecOption {
	return func(s *stdio) {
		s.stderr = w
	}
}

// newstdio is a command's stdio: no input, and output discarded, unless options say otherwise.
func newstdio(options ...ExecOption) stdio {
	return langx.Clone(stdio{stdout: io.Discard, stderr: io.Discard}, options...)
}

// cmdline is the machine's kernel command line. Container images hold no udev to announce the
// console, so systemd starts no getty on it: one would wait out its device's timeout before the
// machine boots. systemd-fstab-generator mounts the virtio-fs devices from systemd.mount-extra, and
// systemd-network-generator configures the machine's NAT device for networkd from ip=. authorized is
// the key root accepts over ssh.
func (t Machine) cmdline(authorized string) string {
	var (
		resolv = base64.StdEncoding.EncodeToString([]byte(resolvlink))
		params = []string{
			"console=hvc0",
			"quiet",
			"init=" + t.init,
			"systemd.getty_auto=0",
			"ip=eth0:dhcp",
			"systemd.wants=systemd-network-generator.service",
			"systemd.wants=systemd-networkd.service",
			"systemd.set_credential_binary=tmpfiles.extra:" + resolv,
			"systemd.set_credential_binary=ssh.ephemeral-authorized_keys-all:" + base64.StdEncoding.EncodeToString([]byte(authorized)),
		}
	)

	for _, m := range t.mounts {
		mode := "rw"
		if m.readonly {
			mode = "ro"
		}
		params = append(params, fmt.Sprintf("systemd.mount-extra=%s:%s:virtiofs:%s", m.tag, m.path, mode))
	}

	return strings.Join(params, " ")
}
