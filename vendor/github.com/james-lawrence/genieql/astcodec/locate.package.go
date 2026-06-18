//go:build !wasm

package astcodec

import (
	"go/build"

	"github.com/james-lawrence/genieql/internal/errorsx"
)

// LocatePackage finds a package by its name.
func LocatePackage(importPath, srcDir string, context build.Context, matches func(*build.Package) bool) (pkg *build.Package, err error) {
	pkg, err = context.Import(importPath, srcDir, build.IgnoreVendor&build.ImportComment)
	_, noGoError := err.(*build.NoGoError)
	if err != nil && !noGoError {
		return nil, errorsx.Wrapf(err, "failed to import the package: %s", importPath)
	}

	if pkg != nil && (matches == nil || matches(pkg)) {
		return pkg, nil
	}

	return nil, ErrPackageNotFound
}
