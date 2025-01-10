package modfilex

import (
	"fmt"
	"go/build"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
)

// ErrPackageNotFound returned when the requested package cannot be located
// within the given context.
const ErrPackageNotFound = errorsx.String("package not found")

func FindModules(root string) iter.Seq[string] {
	tree := os.DirFS(root)

	return func(yield func(string) bool) {
		err := fs.WalkDir(tree, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return errorsx.Wrapf(err, "erroring walking tree: %s", filepath.Join(root, path))
			}

			// ignore vendor directory.
			if filepath.Base(path) == "vendor" && d.IsDir() {
				return fs.SkipDir
			}

			// ignore hidden directories. short term hack.
			if strings.HasPrefix(filepath.Base(path), ".") && path != "." && d.IsDir() {
				return fs.SkipDir
			}

			// recurse into directories.
			if d.IsDir() {
				return nil
			}

			if filepath.Base(path) != "go.mod" {
				return nil
			}

			if !yield(filepath.Join(root, path)) {
				return fmt.Errorf("failed to yield module path: %s", filepath.Join(root, path))
			}

			return nil
		})

		errorsx.Log(errorsx.Wrap(err, "unable to find modules"))
	}
}

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

// StrictPackageName only accepts packages that are an exact match.
func StrictPackageName(name string) func(*build.Package) bool {
	return func(pkg *build.Package) bool {
		return pkg.Name == name
	}
}

// StrictPackageImport only accepts packages that are an exact match.
func StrictPackageImport(name string) func(*build.Package) bool {
	return func(pkg *build.Package) bool {
		return pkg.ImportPath == name
	}
}
