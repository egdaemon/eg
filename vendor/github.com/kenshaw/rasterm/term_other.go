//go:build !unix

package rasterm

// hasSixelSupport returns true if sixel support is available.
func hasSixelSupport() bool {
	return false
}
