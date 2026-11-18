//go:build unix

package ttyx

import (
	"os"
	"syscall"
)

func sigwinch() (os.Signal, bool) {
	return syscall.SIGWINCH, true
}
