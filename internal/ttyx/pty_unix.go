//go:build linux || darwin

package ttyx

import (
	"os"

	"golang.org/x/sys/unix"
)

// resize gives the pseudo terminal of parent the window size of the terminal ctl.
func resize(ctl, parent *os.File) error {
	var (
		ws  *unix.Winsize
		err error
	)

	if err = control(ctl, func(fd int) (err error) {
		ws, err = unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
		return err
	}); err != nil {
		return err
	}

	return control(parent, func(fd int) error {
		return unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, ws)
	})
}

// control runs fn with the descriptor of f without f.Fd(), which would put a pollable file in blocking mode.
func control(f *os.File, fn func(fd int) error) error {
	rc, err := f.SyscallConn()
	if err != nil {
		return err
	}

	var ferr error
	if err = rc.Control(func(fd uintptr) { ferr = fn(int(fd)) }); err != nil {
		return err
	}

	return ferr
}
