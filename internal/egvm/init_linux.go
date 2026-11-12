package egvm

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"github.com/egdaemon/eg/internal/errorsx"
	"golang.org/x/sys/unix"
)

// The names Boot starts this program under, in the machine's user and mount namespaces. stagemap
// waits for the machine's ids, then starts again as stageroot: a program started before its user
// namespace maps its ids holds no capabilities in it, and starting again as the namespace's root
// restores them. stageroot mounts the root and starts stageinit in namespaces of its own, whose
// network pasta connects to the host's, as rootless podman runs pasta beside a container.
// stageinit execs the root's init; before it does it starts stageagent, which serves commands
// inside the root.
const (
	stagemap   = "egvm-map"
	stageroot  = "egvm-root"
	stageinit  = "egvm-init"
	stageagent = "egvm-agent"
)

// The descriptors Boot hands stagemap and stageroot: the pipe Boot closes once it maps the
// machine's ids, the socket stageroot sends the /dev/fuse descriptor of the root's mount back over,
// and the agent's listener. The kernel mounts a FUSE descriptor only in the user namespace that
// opened it, so stageroot opens it, and Boot serves it.
const (
	fdmapped  = 3
	fdmounted = 4
	fdagent   = 5
)

// The descriptors stageroot hands stageinit, and stageinit the agent: the agent's listener, then
// for stageinit the pipe stageroot closes once pasta connects the machine's network, and for the
// agent the pipe stageinit closes when it execs the root's init, by when the agent's root is the
// machine's.
const (
	fdlistener  = 3
	fdnetworked = 4
	fdswitched  = 4
)

// credentials is where the root's systemd reads the system credentials the machine passes it.
const credentials = "/run/egvm/credentials"

func init() {
	var (
		err error
	)

	switch os.Args[0] {
	case stagemap:
		err = waitmapped()
	case stageroot:
		err = boot(os.Args[1], os.Args[2], os.Args[3], os.Args[4:])
	case stageinit:
		err = start(os.Args[1], os.Args[2])
	case stageagent:
		err = serveagent(os.Args[1])
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
// mounted on for pasta. shares are the directories the machine mounts, as triples of the host
// directory, the path within the root, and whether it is read only.
func boot(root string, rootinit string, netns string, shares []string) (err error) {
	var (
		machine   *exec.Cmd
		networked [2]*os.File
		dev       *os.File
		stop      = make(chan os.Signal, 1)
		stopping  = make(chan struct{})
	)

	// starting again as the namespace's root cleared the signal Boot asked for when it exits.
	signal.Notify(stop, unix.SIGTERM)
	if err = unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGTERM), 0, 0, 0); err != nil {
		return err
	}

	for _, fd := range []int{fdmapped, fdmounted, fdagent} {
		unix.CloseOnExec(fd)
	}

	if err = unix.Mount("", "/", "", unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("unable to make the machine's mounts private: %w", err)
	}
	if dev, err = os.OpenFile("/dev/fuse", os.O_RDWR, 0); err != nil {
		return fmt.Errorf("unable to open /dev/fuse: %w", err)
	}
	if err = unix.Mount("egiso", root, "fuse", 0, fmt.Sprintf("fd=%d,rootmode=40000,user_id=0,group_id=0,allow_other,default_permissions", dev.Fd())); err != nil {
		return fmt.Errorf("unable to mount the root: %w", err)
	}
	if err = unix.Sendmsg(fdmounted, []byte{1}, unix.UnixRights(int(dev.Fd())), nil, 0); err != nil {
		return fmt.Errorf("unable to hand over the root's mount: %w", err)
	}
	if err = dev.Close(); err != nil {
		return err
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
	} {
		if err = os.WriteFile(filepath.Join(root, credentials, c.name), []byte(c.value), 0400); err != nil {
			return err
		}
	}
	for i := 0; i+2 < len(shares); i += 3 {
		if err = share(shares[i], filepath.Join(root, shares[i+1]), shares[i+2] == strconv.FormatBool(true)); err != nil {
			return err
		}
	}

	if networked[0], networked[1], err = os.Pipe(); err != nil {
		return err
	}

	// the machine dies with the thread that starts it, so that thread never exits.
	runtime.LockOSThread()
	machine = &exec.Cmd{
		Path:       "/proc/self/exe",
		Args:       []string{stageinit, root, rootinit},
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		ExtraFiles: []*os.File{os.NewFile(fdagent, "agent"), networked[0]},
		SysProcAttr: &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNET | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWCGROUP,
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

// start execs rootinit in root once pasta connects the machine's network, after mounting the
// filesystems its own pid, network and ipc namespaces back and starting the agent.
func start(root string, rootinit string) (err error) {
	var (
		agent    *exec.Cmd
		switched [2]int
	)

	if _, err = io.Copy(io.Discard, os.NewFile(fdnetworked, "networked")); err != nil {
		return err
	}

	for _, m := range []struct {
		source string
		target string
		fstype string
		flags  uintptr
	}{
		{source: "proc", target: "proc", fstype: "proc", flags: unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC},
		{source: "sysfs", target: "sys", fstype: "sysfs", flags: unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_RDONLY},
	} {
		if err = mountat(m.source, filepath.Join(root, m.target), m.fstype, m.flags, ""); err != nil {
			return err
		}
	}

	// the write end is a bare descriptor, which only the exec of the root's init closes.
	if err = unix.Pipe2(switched[:], unix.O_CLOEXEC); err != nil {
		return err
	}
	agent = &exec.Cmd{Path: "/proc/self/exe", Args: []string{stageagent, rootinit}, Stdout: os.Stdout, Stderr: os.Stderr, ExtraFiles: []*os.File{os.NewFile(fdlistener, "agent"), os.NewFile(uintptr(switched[0]), "switched")}}
	err = agent.Start()
	agent.ExtraFiles[1].Close()
	if err != nil {
		return fmt.Errorf("unable to start the agent: %w", err)
	}
	unix.CloseOnExec(fdlistener)
	unix.CloseOnExec(fdnetworked)

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

	return fmt.Errorf("unable to exec %s: %w", rootinit, unix.Exec(rootinit, []string{rootinit}, []string{"container=egvm", "CREDENTIALS_DIRECTORY=" + credentials}))
}

// devices mounts the root's /dev: the host's character devices containers are given, with fuse, as
// the runner gives its containers for eg module's bindfs, and its own pseudo terminals.
func devices(root string) (err error) {
	var (
		dev = filepath.Join(root, "dev")
	)

	if err = mountat("tmpfs", dev, "tmpfs", unix.MS_NOSUID|unix.MS_NOEXEC, "mode=755"); err != nil {
		return err
	}
	for _, name := range []string{"null", "zero", "full", "random", "urandom", "tty", "fuse"} {
		if err = os.WriteFile(filepath.Join(dev, name), nil, 0666); err != nil {
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

// share binds the host directory dir at target, read only if readonly says so.
func share(dir string, target string, readonly bool) (err error) {
	var (
		st unix.Statfs_t
	)

	if err = os.MkdirAll(target, 0755); err != nil {
		return err
	}
	if err = unix.Mount(dir, target, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("unable to share %s: %w", dir, err)
	}
	if !readonly {
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

// serveagent serves commands on the listener stageinit hands it, once stageinit switches the root and
// rootinit answers them. Switching the root moves the agent's along with it.
func serveagent(rootinit string) (err error) {
	if _, err = io.Copy(io.Discard, os.NewFile(fdswitched, "switched")); err != nil {
		return err
	}
	if err = unix.Chdir("/"); err != nil {
		return err
	}
	if err = Answering(rootinit); err != nil {
		return err
	}
	return Serve(fdlistener)
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
