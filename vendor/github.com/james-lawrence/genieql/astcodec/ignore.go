package astcodec

import (
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Ignored reports whether pkg matches any of the ignore patterns. Each
// pattern may be a package import path or a filesystem directory (resolved
// relative to workingdir when not absolute); matching a directory or import
// path also matches everything nested beneath it.
func Ignored(workingdir string, ignore []string, pkg *packages.Package) bool {
	pkgdir := filepath.Clean(pkg.Dir)

	for _, pattern := range ignore {
		if pkg.PkgPath == pattern || strings.HasPrefix(pkg.PkgPath, pattern+"/") {
			return true
		}

		dir := pattern
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(workingdir, dir)
		}
		dir = filepath.Clean(dir)

		if pkgdir == dir || strings.HasPrefix(pkgdir, dir+string(filepath.Separator)) {
			return true
		}
	}

	return false
}
