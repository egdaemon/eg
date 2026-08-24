package genieql

import (
	"go/build"
	"os"
	"strings"

	"github.com/james-lawrence/genieql/internal/errorsx"
	"github.com/james-lawrence/genieql/internal/langx"
)

func currentPackage(bctx build.Context, path string, dir string) *build.Package {
	pkg, err := bctx.Import(".", dir, build.IgnoreVendor)
	errorsx.MaybePanic(errorsx.Wrapf(err, "failed to load package for %s", dir))
	pkg.ImportPath = path

	return pkg
}

func NewBuildInfo() (bi BuildInfo, err error) {
	var (
		workingDir string
		modname    string
		modroot    string
	)

	if workingDir, err = os.Getwd(); err != nil {
		return bi, err
	}

	if modroot, err = FindModuleRoot(workingDir); err != nil {
		return bi, err
	}

	if modname, err = FindModulePath(workingDir); err != nil {
		return bi, err
	}

	return BuildInfo{
		Build:      build.Default,
		WorkingDir: workingDir,
		ModuleRoot: modroot,
		Module:     modname,
		CurrentPKG: currentPackage(build.Default, strings.Replace(workingDir, modroot, modname, -1), workingDir),
	}, nil
}

type BuildInfo struct {
	Build      build.Context
	Verbosity  int
	WorkingDir string
	ModuleRoot string
	Module     string
	CurrentPKG *build.Package
}

// CurrentPackageDir returns the directory of the current package if any.
// returns an empty string otherwise.
func (t BuildInfo) CurrentPackageDir() string {
	return langx.Autoderef(t.CurrentPKG).Dir
}

// CurrentPackageImport returns the import path for the current package if any.
// returns an empty string otherwise.
func (t BuildInfo) CurrentPackageImport() string {
	return langx.Autoderef(t.CurrentPKG).ImportPath
}
