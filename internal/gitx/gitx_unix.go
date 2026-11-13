//go:build !windows

package gitx

import (
	"os"
	"strconv"
	"syscall"
)

// fileOwnerID returns the id of the user owning the file, false when the platform doesn't report one.
func fileOwnerID(info os.FileInfo) (string, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", false
	}

	return strconv.FormatUint(uint64(st.Uid), 10), true
}
