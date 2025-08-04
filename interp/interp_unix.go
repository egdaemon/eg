//go:build unix

package interp

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// want to ensure we dont munge stdin
func nonBlocking(fd uintptr) bool {
	// Use FcntlInt to retrieve the file status flags for the descriptor.
	// F_GETFL is the command to get the flags, and the third argument (0)
	// is ignored for this operation.
	flags, err := unix.FcntlInt(fd, unix.F_GETFL, 0)
	if err != nil {
		// Panic on error, as requested. This assumes the file descriptor
		// should be valid and an error indicates a critical issue.
		panic(fmt.Errorf("F_GETFL failed: %w", err))
	}

	// Check if the O_NONBLOCK flag is set using a bitwise AND.
	// unix.O_NONBLOCK is a bitmask constant.
	// If the bit is set, the result of the AND operation will be non-zero.
	return (flags & unix.O_NONBLOCK) != 0
}
