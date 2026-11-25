//go:build !wasip1 && !windows

package wasip1syscall

import "syscall"

// the wasi values for the socket level options (constants.go) are what the guest sends. the host
// translates them to its system values before they reach the kernel, the same way ErrnoTranslate
// goes from host to wasi: SO_RCVTIMEO is 20 on linux but 0x1006 on darwin.
var sockoptnames = map[int]int{
	SO_REUSEADDR: syscall.SO_REUSEADDR,
	SO_ERROR:     syscall.SO_ERROR,
	SO_BROADCAST: syscall.SO_BROADCAST,
	SO_RCVTIMEO:  syscall.SO_RCVTIMEO,
	SO_SNDTIMEO:  syscall.SO_SNDTIMEO,
}

// SockoptTranslate maps a wasi socket option level and name to the host's system values.
// options at other levels, and unmapped names, are passed through unchanged.
func SockoptTranslate(level, name int) (int, int) {
	if level != SOL_SOCKET {
		return level, name
	}

	if native, ok := sockoptnames[name]; ok {
		name = native
	}

	return syscall.SOL_SOCKET, name
}
