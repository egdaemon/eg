// Package egarchx maps GOARCH architecture names (amd64, arm64, 386, arm)
// to various third-party architecture naming conventions.
package egarchx

import (
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

// Host arch
func Host() string {
	return egenv.String("amd64", eg.EnvComputeArch)
}

// POSIX see POSIXFrom
func POSIX() string {
	return DartFrom(egenv.String("amd64", eg.EnvComputeArch))
}

// POSIXFrom maps GOARCH/dpkg architecture names (amd64, arm64, 386, arm)
// to POSIXFrom uname -m machine hardware names (x86_64, aarch64, i686, armhf).
func POSIXFrom(arch string) string {
	switch arch {
	case "arm64":
		return "aarch64"
	case "386":
		return "i686"
	case "arm":
		return "armhf"
	default: // amd64
		return "x86_64"
	}
}

// Dart see DartFrom provided the current runtime.GOARCH
func Dart() string {
	return DartFrom(egenv.String("amd64", eg.EnvComputeArch))
}

// DartFrom maps GOARCH/dpkg architecture names (386, amd64, arm, arm64)
// to dart names (ia32, x64, arm, arm64).
func DartFrom(arch string) string {
	switch arch {
	case "arm64":
		return "arm64"
	case "386":
		return "ia32"
	case "arm":
		return "arm"
	default: // amd64
		return "x64"
	}
}
