//go:build linux

// Command vminit is the init of an egvm machine. It loads the kernel modules in /modules in name
// order, mounts the virtio-fs share tagged root, runs its arguments (the words after -- on the
// kernel's command line) chrooted into the share, prints "egvm: exit <code>" and powers off.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

func main() {
	fmt.Printf("egvm: exit %d\n", run(os.Args[1:]))
	unix.Sync()
	if err := unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF); err != nil {
		fmt.Println("egvm: unable to power off", err)
	}
}

func run(args []string) int {
	var (
		exit *exec.ExitError
	)

	for _, m := range []struct{ source, target, fstype string }{
		{"proc", "/proc", "proc"},
		{"sysfs", "/sys", "sysfs"},
		{"devtmpfs", "/dev", "devtmpfs"},
	} {
		if err := mount(m.source, m.target, m.fstype); err != nil {
			fmt.Println("egvm:", err)
			return 125
		}
	}

	modules, err := filepath.Glob("/modules/*.ko")
	if err != nil {
		fmt.Println("egvm:", err)
		return 125
	}
	for _, m := range modules {
		if err = load(m); err != nil {
			fmt.Println("egvm:", err)
			return 125
		}
	}

	if err = mount("root", "/root", "virtiofs"); err != nil {
		fmt.Println("egvm:", err)
		return 125
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = "/"
	cmd.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "HOME=/root"}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: "/root"}
	if err = cmd.Run(); errors.As(err, &exit) {
		return exit.ExitCode()
	} else if err != nil {
		fmt.Println("egvm:", err)
		return 126
	}
	return 0
}

func mount(source string, target string, fstype string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	if err := unix.Mount(source, target, fstype, 0, ""); err != nil {
		return fmt.Errorf("unable to mount %s at %s: %w", fstype, target, err)
	}
	return nil
}

func load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = unix.FinitModule(int(f.Fd()), "", 0); err != nil && !errors.Is(err, unix.EEXIST) {
		return fmt.Errorf("unable to load %s: %w", path, err)
	}
	return nil
}
