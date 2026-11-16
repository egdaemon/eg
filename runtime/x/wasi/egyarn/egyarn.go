// Package egyarn has supporting functions for configuring the environment for running yarn berry for caching.
// in the future we may support previous versions.
package egyarn

import (
	"path/filepath"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func CacheDirectory(dirs ...string) string {
	return egenv.DirectoryCache(_eg.DefaultModuleDirectory(), "yarn", filepath.Join(dirs...))
}

// attempt to build the yarn environment that properly
func env() ([]string, error) {
	return envx.Build().
		Var("COREPACK_ENABLE_DOWNLOAD_PROMPT", "0").
		Var("COREPACK_HOME", egenv.DirectoryCache(_eg.DefaultModuleDirectory(), "corepack")).
		Var("YARN_CACHE_FOLDER", CacheDirectory()).
		Var("YARN_ENABLE_GLOBAL_CACHE", false).
		Environ()
}

// attempt to build the yarn environment that properly sets up caching.
//
// only the variables it sets are returned, the process environment is not copied. to inherit it, start from shell.Env()
// and add these after it, later entries win:
//
//	shell.Env().EnvironFrom(egyarn.Env()...)
func Env() []string {
	return errorsx.Must(env())
}

// Create a shell runtime that properly
// sets up the yarn environment for caching.
func Runtime() shell.Command {
	return shell.Runtime().EnvironFrom(Env()...)
}
