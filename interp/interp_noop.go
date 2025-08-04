//go:build !unix

package interp

// want to ensure we dont munge stdin
func nonBlocking(fd uintptr) bool {
	return false
}
