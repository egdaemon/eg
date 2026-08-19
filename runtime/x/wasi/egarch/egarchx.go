// Package egarch maps GOARCH architecture names (amd64, arm64, 386, arm)
// to various third-party architecture naming conventions.
package egarch

// POSIX maps GOARCH/dpkg architecture names (amd64, arm64, 386, arm)
// to POSIX uname -m machine hardware names (x86_64, aarch64, i686, armhf).
func POSIX(arch string) string {
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

// Dart maps GOARCH/dpkg architecture names (386, amd64, arm, arm64)
// to dart names (ia32, x64, arm, arm64).
func Dart(arch string) string {
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
