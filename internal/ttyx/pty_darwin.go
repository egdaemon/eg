package ttyx

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// openpty allocates a pseudo terminal. the parent is the side the mux holds (/dev/ptmx), the child is the terminal handed to
// a command (/dev/ttysNNN). the parent is left pollable, so closing it stops a read in progress.
func openpty() (parent, child *os.File, err error) {
	if parent, err = os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0); err != nil {
		return nil, nil, err
	}

	var name string
	if err = control(parent, func(fd int) error {
		// the child device is owned by the user and unlocked before it can be opened.
		if err := unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
			return err
		}

		if err := unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
			return err
		}

		// x/sys has no helper for the name, the ioctl fills a buffer of MAXPATHLEN (128) with a NUL terminated path.
		var buf [128]byte
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(unix.TIOCPTYGNAME), uintptr(unsafe.Pointer(&buf[0]))); errno != 0 {
			return errno
		}

		for i, b := range buf {
			if b == 0 {
				name = string(buf[:i])
				return nil
			}
		}

		return syscall.ENAMETOOLONG
	}); err != nil {
		parent.Close()
		return nil, nil, err
	}

	if child, err = os.OpenFile(name, os.O_RDWR|unix.O_NOCTTY, 0); err != nil {
		parent.Close()
		return nil, nil, err
	}

	return parent, child, nil
}
