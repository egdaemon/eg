// Package egterraform has supporting functions for configuring the environment for running terraform commands
// within eg.
package egterraform

import (
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func CacheDirectory(dirs ...string) string {
	return egenv.CacheDirectory("eg.terraform", filepath.Join(dirs...))
}

// attempt to build the yarn environment that properly
func Env() ([]string, error) {
	return envx.Build().FromEnv(os.Environ()...).
		Var("TF_PLUGIN_CACHE_DIR", CacheDirectory("plugins")).
		Var("TF_IN_AUTOMATION", envx.VarBool(true)).
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
