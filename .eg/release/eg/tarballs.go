// Package eg holds what the release workloads of eg share, chiefly the names of the tarballs they publish.
package eg

import (
	"fmt"

	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
)

// Tar is the git patterned name, without an extension, of the eg tarball for the os and architecture.
func Tar(os, arch string) string {
	return egtarball.GitPattern(fmt.Sprintf("eg.%s.%s", os, arch))
}

// TarDarwin is the .tar.xz keg archive of eg for darwin arm64 consumed by the homebrew formula.
func TarDarwin() string {
	return egtarball.Tarxz(Tar("brew.darwin", "arm64"))
}

// TarLinux is the .tar.gz release archive of eg for linux on the given architecture.
func TarLinux(arch string) string {
	return egtarball.Targz(Tar("linux", arch))
}
