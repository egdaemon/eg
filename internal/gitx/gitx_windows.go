//go:build windows

package gitx

import "os"

// fileOwnerID is unsupported on windows, files don't carry a uid.
func fileOwnerID(os.FileInfo) (string, bool) {
	return "", false
}
