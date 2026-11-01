//go:build unix

package ttyx

import (
	"os"
	"syscall"
)

// dup gives the caller its own descriptor for f, so closing it leaves f open.
func dup(f *os.File) (*os.File, error) {
	fd, err := syscall.Dup(int(f.Fd()))
	if err != nil {
		return nil, err
	}

	syscall.CloseOnExec(fd)

	return os.NewFile(uintptr(fd), f.Name()), nil
}
