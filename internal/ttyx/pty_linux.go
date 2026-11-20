package ttyx

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// openpty allocates a pseudo terminal. the parent is the side the mux holds (/dev/ptmx), the child is the terminal handed to
// a command (/dev/pts/N). the parent is left pollable, so closing it stops a read in progress.
func openpty() (parent, child *os.File, err error) {
	if parent, err = os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0); err != nil {
		return nil, nil, err
	}

	var n int
	if err = control(parent, func(fd int) error {
		if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
			return err
		}
		n, err = unix.IoctlGetInt(fd, unix.TIOCGPTN)
		return err
	}); err != nil {
		parent.Close()
		return nil, nil, err
	}

	if child, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0); err != nil {
		parent.Close()
		return nil, nil, err
	}

	return parent, child, nil
}
