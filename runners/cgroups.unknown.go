//go:build !linux

package runners

// IsolateCgroups is linux only, see cgroups.linux.go.
func IsolateCgroups(runid string) error {
	return nil
}
