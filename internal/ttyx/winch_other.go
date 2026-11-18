//go:build !unix

package ttyx

import "os"

// sigwinch is unavailable, the size of a session is only set when it is created.
func sigwinch() (os.Signal, bool) {
	return nil, false
}
