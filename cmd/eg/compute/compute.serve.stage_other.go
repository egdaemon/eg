//go:build !darwin

package compute

// stageHostDylibs is a no-op outside darwin: the linux eg binary links
// either against system libs or statically-bundled ones, with nothing to
// gather from /opt/homebrew.
func stageHostDylibs(_, _ string) error {
	return nil
}
