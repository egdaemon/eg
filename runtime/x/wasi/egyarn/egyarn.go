// Package egyarn has supporting functions for configuring the environment for running yarn berry for caching.
// in the future we may support previous versions.
package egyarn

import (
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func CacheDirectory(dirs ...string) string {
	return egenv.CacheDirectory(".eg", "yarn", filepath.Join(dirs...))
}

// attempt to build the yarn environment that properly
func Env() ([]string, error) {
	return envx.Build().FromEnv(os.Environ()...).
		Var("COREPACK_HOME", egenv.CacheDirectory(".eg", "corepack")).
		Var("YARN_CACHE_FOLDER", CacheDirectory()).
		Var("YARN_ENABLE_GLOBAL_CACHE", envx.VarBool(false)).
		Environ()
}

// Create a shell runtime that properly
// sets up the yarn environment for caching.
func Runtime() shell.Command {
	return shell.Runtime().
		EnvironFrom(
			errorsx.Must(Env())...,
		)
}
