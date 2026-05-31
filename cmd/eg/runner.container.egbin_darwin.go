//go:build darwin

package main

// resolveContainerEGBin returns the in-container eg path. Inside a macvm
// guest there is no podman and no container layer, so the path is left
// empty — any FFI call that triggers container exec will surface its own
// error rather than panicking at startup.
func resolveContainerEGBin() string {
	return ""
}
