//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package ttyx

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// devkey identifies the terminal device f is open on, every descriptor of the device shares its mode.
func devkey(f *os.File) (string, error) {
	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("rdev:%d", uint64(st.Rdev)), nil
	}

	return "name:" + f.Name(), nil
}

// cooked turns output processing back on after raw mode, and optionally signal generation.
func cooked(fd int, keepsig bool) error {
	t, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return err
	}

	t.Oflag |= unix.OPOST | unix.ONLCR
	if keepsig {
		t.Lflag |= unix.ISIG
	}

	return unix.IoctlSetTermios(fd, ioctlSetTermios, t)
}
