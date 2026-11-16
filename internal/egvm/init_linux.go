package egvm

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/egdaemon/eg/internal/errorsx"
	"go.podman.io/common/pkg/cgroups"
	"golang.org/x/sys/unix"
)

// The names Boot starts this program under, in the machine's user and mount namespaces. stagemap
// waits for the machine's ids, then starts again as stageroot: a program started before its user
// namespace maps its ids holds no capabilities in it, and starting again as the namespace's root
// restores them. stageroot mounts the root and starts stagenet in a network namespace of its own,
// which pasta connects to the host's, as rootless podman runs pasta beside a container. stagenet
// loads the machine's firewall and starts stageinit in a user namespace of its own and the machine's
// other namespaces, so the machine has no say over its network namespace, but owns any it makes.
// stageinit execs the root's init.
const (
	stagemap  = "egvm-map"
	stageroot = "egvm-root"
	stagenet  = "egvm-net"
	stageinit = "egvm-init"
)

// The descriptors Boot hands stagemap and stageroot: the pipe Boot closes once it maps the
// machine's ids, and the socket stageroot sends the /dev/fuse descriptors of the root's mount and
// then of each share Boot serves back over. The kernel mounts a FUSE descriptor only in the user
// namespace that opened it, so stageroot opens it, and Boot serves it.
const (
	fdmapped  = 3
	fdmounted = 4
)

// How stageroot mounts a share: bound read write or read only, or over FUSE, which Boot serves.
const (
	sharerw   = "rw"
	sharero   = "ro"
	sharefuse = "fuse"
)

// The descriptors stageroot hands stagenet: the pipe it closes once pasta connects the machine's
// network, and the payload cgroup the machine's init starts in.
const (
	fdnetworked = 3
	fdpayload   = 4
)

// credentials is where the root's systemd reads the system credentials the machine passes it.
const credentials = "/run/egvm/credentials"

// nft is the root's nftables tool, which loads the machine's firewall.
const nft = "/usr/sbin/nft"

func init() {
	var (
		err error
	)

	switch os.Args[0] {
	case stagemap:
		err = waitmapped()
	case stageroot:
		err = boot(os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6:])
	case stagenet:
		err = network(os.Args[1], os.Args[2], os.Args[3])
	case stageinit:
		err = start(os.Args[1], os.Args[2])
	default:
		return
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "egvm:", os.Args[0], err)
		os.Exit(1)
	}
	os.Exit(0)
}

// waitmapped starts the program again as stageroot once Boot closes its end of fdmapped.
func waitmapped() error {
	if _, err := io.Copy(io.Discard, os.NewFile(fdmapped, "mapped")); err != nil {
		return err
	}
	return unix.Exec("/proc/self/exe", append([]string{stageroot}, os.Args[1:]...), os.Environ())
}

// boot mounts the root at root over FUSE and readies it as a container runtime readies an image,
// then runs the machine's init until it exits. netns is the file the init's network namespace is
// mounted on for pasta. authorized is the key root accepts over ssh, and ruleset the machine's
// firewall. shares are the directories the machine mounts, as triples of the host directory, the path
// within the root, and how it mounts them.
func boot(root string, rootinit string, netns string, authorized string, ruleset string, shares []string) (err error) {
	var (
		machine   *exec.Cmd
		networked [2]*os.File
		payload   *os.File
		stop      = make(chan os.Signal, 1)
		stopping  = make(chan struct{})
	)

	// starting again as the namespace's root cleared the signal Boot asked for when it exits.
	signal.Notify(stop, unix.SIGTERM)
	if err = unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGTERM), 0, 0, 0); err != nil {
		return err
	}

	for _, fd := range []int{fdmapped, fdmounted} {
		unix.CloseOnExec(fd)
	}

	if err = unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("unable to make the machine's mounts private: %w", err)
	}
	if err = fusemount("egiso", root, "allow_other,default_permissions"); err != nil {
		return fmt.Errorf("unable to mount the root: %w", err)
	}

	if err = devices(root); err != nil {
		return err
	}
	if err = mountat("tmpfs", filepath.Join(root, "run"), "tmpfs", unix.MS_NOSUID|unix.MS_NODEV, "mode=755"); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(root, credentials), 0700); err != nil {
		return err
	}
	for _, c := range []struct {
		name  string
		value string
	}{
		{name: "tmpfiles.extra", value: resolvlink},
		{name: "network.dns", value: dnsforward},
		{name: "ssh.ephemeral-authorized_keys-all", value: authorized},
	} {
		if err = os.WriteFile(filepath.Join(root, credentials, c.name), []byte(c.value), 0400); err != nil {
			return err
		}
	}
	for i := 0; i+2 < len(shares); i += 3 {
		if err = share(shares[i], filepath.Join(root, shares[i+1]), shares[i+2]); err != nil {
			return err
		}
	}

	if payload, err = delegate(); err != nil {
		return fmt.Errorf("unable to hand the machine its cgroup: %w", err)
	}
	defer payload.Close()

	if networked[0], networked[1], err = os.Pipe(); err != nil {
		return err
	}

	// the machine dies with the thread that starts it, so that thread never exits.
	runtime.LockOSThread()
	machine = &exec.Cmd{
		Path:       "/proc/self/exe",
		Args:       []string{stagenet, root, rootinit, ruleset},
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		ExtraFiles: []*os.File{networked[0], payload},
		SysProcAttr: &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWNET,
			Pdeathsig:  syscall.SIGKILL,
		},
	}
	err = machine.Start()
	networked[0].Close()
	if err != nil {
		return fmt.Errorf("unable to start the machine's init: %w", err)
	}

	// the machine's pid namespace ends with its init; pasta exits once its namespace file goes, and
	// the root's FUSE session once nothing mounts it.
	defer func() {
		if machine.ProcessState == nil {
			errorsx.Log(machine.Process.Kill())
			errorsx.Log(machine.Wait())
		}
		errorsx.Log(unix.Unmount(netns, unix.MNT_DETACH))
		errorsx.Log(os.Remove(netns))
		errorsx.Log(unix.Unmount(root, unix.MNT_DETACH))
	}()
	go func() {
		<-stop
		close(stopping)
		machine.Process.Kill()
	}()

	if err = unix.Mount(fmt.Sprintf("/proc/%d/ns/net", machine.Process.Pid), netns, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("unable to mount the machine's network namespace: %w", err)
	}
	// pasta forwards no ports either way, as podman runs it: by default it binds inside the machine
	// the ports the host listens on, taking them from the root's own services.
	if b, cause := exec.Command("pasta", "--config-net", "--dns-forward", dnsforward, "-t", "none", "-u", "none", "-T", "none", "-U", "none", "--quiet", "--netns", netns).CombinedOutput(); cause != nil {
		return fmt.Errorf("unable to connect the machine's network: %w: %s", cause, b)
	}
	networked[1].Close()

	// the machine's init is killed when Boot's caller stops the machine, which is no failure.
	err = machine.Wait()
	select {
	case <-stopping:
		return nil
	default:
		return err
	}
}

// delegate moves this process from the machine's scope into the scope's supervisor cgroup, hands the
// scope's controllers to its children, and opens the payload cgroup for the machine's init to start
// in, as systemd-nspawn keeps itself apart from its container. A cgroup holding processes can not
// hand its children controllers, so the machine's systemd could hand its own none.
func delegate() (_ *os.File, err error) {
	var (
		scope       string
		payload     string
		controllers string
		limit       string
	)

	if scope, err = cgroups.GetOwnCgroup(); err != nil {
		return nil, err
	}
	scope = filepath.Join("/sys/fs/cgroup", scope)
	payload = filepath.Join(scope, "payload")
	if err = cgroups.MoveUnderCgroupSubtree("supervisor"); err != nil {
		return nil, err
	}
	if controllers, err = cgroups.ReadFile(scope, "cgroup.controllers"); err != nil {
		return nil, err
	}
	for _, c := range strings.Fields(controllers) {
		if err = cgroups.WriteFile(scope, "cgroup.subtree_control", "+"+c); err != nil {
			return nil, err
		}
	}
	if err = os.Mkdir(payload, 0755); err != nil {
		return nil, err
	}
	// the machine sees only its own cgroup, so the payload repeats the scope's limits for it to read.
	for _, file := range []string{"cpu.max", "memory.max"} {
		if limit, err = cgroups.ReadFile(scope, file); err != nil {
			return nil, err
		}
		if err = cgroups.WriteFile(payload, file, limit); err != nil {
			return nil, err
		}
	}
	return os.Open(payload)
}

// network loads ruleset, the machine's firewall, with the root's nft once pasta connects the
// machine's network, and mounts the root's /sys, which only the network namespace's user namespace
// may. It then runs stageinit in the payload cgroup until it exits, as root of a user namespace mapping
// each id to its own: root in the machine changes no firewall, as the namespace that owns its
// network is this one, and administers the networks it makes for its own machines and containers.
func network(root string, rootinit string, ruleset string) (err error) {
	var (
		machine  *exec.Cmd
		uids     []syscall.SysProcIDMap
		gids     []syscall.SysProcIDMap
		firewall = exec.Command(nft, "-f", "-")
	)

	if _, err = io.Copy(io.Discard, os.NewFile(fdnetworked, "networked")); err != nil {
		return err
	}
	for _, fd := range []int{fdnetworked, fdpayload} {
		unix.CloseOnExec(fd)
	}

	firewall.Stdin = strings.NewReader(ruleset)
	firewall.SysProcAttr = &syscall.SysProcAttr{Chroot: root}
	if b, cause := firewall.CombinedOutput(); cause != nil {
		return fmt.Errorf("unable to load the machine's firewall: %w: %s", cause, b)
	}

	if err = mountat("sysfs", filepath.Join(root, "sys"), "sysfs", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RDONLY, ""); err != nil {
		return err
	}

	if uids, err = identity("/proc/self/uid_map"); err != nil {
		return err
	}
	if gids, err = identity("/proc/self/gid_map"); err != nil {
		return err
	}

	// the machine dies with the thread that starts it, so that thread never exits.
	runtime.LockOSThread()
	machine = &exec.Cmd{
		Path:   "/proc/self/exe",
		Args:   []string{stageinit, root, rootinit},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		SysProcAttr: &syscall.SysProcAttr{
			Cloneflags:                 syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS | syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWCGROUP,
			UidMappings:                uids,
			GidMappings:                gids,
			GidMappingsEnableSetgroups: true,
			Pdeathsig:                  syscall.SIGKILL,
			UseCgroupFD:                true,
			CgroupFD:                   fdpayload,
		},
	}
	return machine.Run()
}

// identity maps each id of this process's user namespace to itself, as podman nested with --userns
// host maps a container's, from file, its uid_map or gid_map.
func identity(file string) (ids []syscall.SysProcIDMap, err error) {
	var (
		b      []byte
		id     int
		parent int
		size   int
	)

	if b, err = os.ReadFile(file); err != nil {
		return nil, err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if _, err = fmt.Sscan(line, &id, &parent, &size); err != nil {
			return nil, fmt.Errorf("unable to read %s: %w", file, err)
		}
		ids = append(ids, syscall.SysProcIDMap{ContainerID: id, HostID: id, Size: size})
	}
	return ids, nil
}

// start execs rootinit in root, after mounting the proc filesystem of its own pid namespace.
func start(root string, rootinit string) (err error) {
	// a user namespace of its own cleared the signal stagenet asked for when it exits.
	if err = unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGKILL), 0, 0, 0); err != nil {
		return err
	}

	// the root's mount came from the user namespace above, which locks it, and pivot_root moves no
	// locked mount, so the root moves bound onto itself, as crun binds a container's.
	if err = unix.Mount(root, root, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("unable to bind the root: %w", err)
	}
	if err = mountat("proc", filepath.Join(root, "proc"), "proc", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, ""); err != nil {
		return err
	}

	if err = unix.Chdir(root); err != nil {
		return err
	}
	if err = unix.PivotRoot(".", "."); err != nil {
		return fmt.Errorf("unable to enter the root: %w", err)
	}
	if err = unix.Unmount(".", unix.MNT_DETACH); err != nil {
		return fmt.Errorf("unable to leave the host's root: %w", err)
	}
	if err = unix.Chdir("/"); err != nil {
		return err
	}

	// raw io is the host's alone, so the root's systemd leaves the kernel's debugging filesystems
	// unmounted, as it does in a rootless podman container.
	if err = unix.Prctl(unix.PR_CAPBSET_DROP, unix.CAP_SYS_RAWIO, 0, 0, 0); err != nil {
		return fmt.Errorf("unable to drop CAP_SYS_RAWIO: %w", err)
	}

	return fmt.Errorf("unable to exec %s: %w", rootinit, unix.Exec(rootinit, []string{rootinit}, []string{"container=" + Container, "CREDENTIALS_DIRECTORY=" + credentials}))
}

// devices mounts the root's /dev: the host's character devices containers are given, with fuse, as
// the runner gives its containers for eg module's bindfs, tun, for pasta to connect the machines the
// machine boots, and its own pseudo terminals.
func devices(root string) (err error) {
	var (
		dev = filepath.Join(root, "dev")
	)

	if err = mountat("tmpfs", dev, "tmpfs", unix.MS_NOSUID|unix.MS_NOEXEC, "mode=755"); err != nil {
		return err
	}
	for _, name := range []string{"null", "zero", "full", "random", "urandom", "tty", "fuse", "net/tun"} {
		if err = mountpoint(filepath.Join(dev, name), false); err != nil {
			return err
		}
		if err = unix.Mount(filepath.Join("/dev", name), filepath.Join(dev, name), "", unix.MS_BIND, ""); err != nil {
			return fmt.Errorf("unable to mount /dev/%s: %w", name, err)
		}
	}
	if err = mountat("devpts", filepath.Join(dev, "pts"), "devpts", unix.MS_NOSUID|unix.MS_NOEXEC, "newinstance,ptmxmode=0666,mode=0620"); err != nil {
		return err
	}
	return os.Symlink("pts/ptmx", filepath.Join(dev, "ptmx"))
}

// fusemount mounts a FUSE filesystem named source at target with the mount options options, and
// hands its /dev/fuse descriptor to Boot to serve.
func fusemount(source string, target string, options string) (err error) {
	var (
		dev *os.File
	)

	if dev, err = os.OpenFile("/dev/fuse", os.O_RDWR, 0); err != nil {
		return fmt.Errorf("unable to open /dev/fuse: %w", err)
	}

	if err = unix.Mount(source, target, "fuse", 0, fmt.Sprintf("fd=%d,rootmode=40000,user_id=0,group_id=0,%s", dev.Fd(), options)); err != nil {
		return errorsx.Compact(err, dev.Close())
	}
	if err = unix.Sendmsg(fdmounted, []byte{1}, unix.UnixRights(int(dev.Fd())), nil, 0); err != nil {
		return errorsx.Compact(fmt.Errorf("unable to hand over the mount: %w", err), dev.Close())
	}
	return dev.Close()
}

// share mounts the host directory or file dir at target as mode says: bound read write or read only,
// or over FUSE, which Boot serves.
func share(dir string, target string, mode string) (err error) {
	var (
		st  unix.Statfs_t
		src os.FileInfo
	)

	if src, err = os.Stat(dir); err != nil {
		return err
	}
	if err = mountpoint(target, src.IsDir()); err != nil {
		return err
	}
	// the kernel checks no modes in a FUSE share: every user writes it as the host user, whose own
	// permissions the host applies.
	if mode == sharefuse {
		if err = fusemount(dir, target, "allow_other"); err != nil {
			return fmt.Errorf("unable to share %s: %w", dir, err)
		}
		return nil
	}
	if err = unix.Mount(dir, target, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("unable to share %s: %w", dir, err)
	}
	if mode != sharero {
		return nil
	}

	// a user namespace can not drop the flags the host mounted dir with, so remounting keeps them.
	if err = unix.Statfs(target, &st); err != nil {
		return err
	}
	locked := uintptr(st.Flags) & (unix.ST_NOSUID | unix.ST_NODEV | unix.ST_NOEXEC)
	if err = unix.Mount("", target, "", unix.MS_REMOUNT|unix.MS_BIND|unix.MS_RDONLY|locked, ""); err != nil {
		return fmt.Errorf("unable to share %s read only: %w", dir, err)
	}
	return nil
}

// mountpoint creates target to mount a directory on, or a file, as podman does for a file it mounts:
// an empty file, in what may be the host's directory.
func mountpoint(target string, dir bool) (err error) {
	var (
		f *os.File
	)

	if dir {
		return os.MkdirAll(target, 0755)
	}
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	if f, err = os.OpenFile(target, os.O_CREATE|os.O_RDONLY, 0755); err != nil {
		return err
	}
	return f.Close()
}

// mountat mounts source at target, creating target first.
func mountat(source string, target string, fstype string, flags uintptr, data string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	if err := unix.Mount(source, target, fstype, flags, data); err != nil {
		return fmt.Errorf("unable to mount %s at %s: %w", fstype, target, err)
	}
	return nil
}
