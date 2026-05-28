//go:build !darwin

package main

import (
	"os/exec"

	"github.com/egdaemon/eg/internal/errorsx"

	"github.com/egdaemon/eg"
)

// resolveContainerEGBin returns the in-container path to the eg binary used
// by nested container runs. The binary must exist at the standard mount
// location on Linux — a missing entry is an operator-misconfiguration that
// the root module shouldn't paper over.
func resolveContainerEGBin() string {
	return errorsx.Must(exec.LookPath(eg.DefaultMountRoot(eg.RuntimeDirectory, eg.BinaryBin)))
}
