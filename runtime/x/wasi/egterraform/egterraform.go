// Package egterraform has supporting functions for configuring the environment for running terraform commands
// within eg.
package egterraform

import (
	"os"
	"path/filepath"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func CacheDirectory(dirs ...string) string {
	return egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "terraform", filepath.Join(dirs...))
}

// attempt to build the terraform environment that properly
func env() ([]string, error) {
	return envx.Build().FromEnv(os.Environ()...).
		Var("TF_PLUGIN_CACHE_DIR", CacheDirectory("plugins")).
		Var("TF_IN_AUTOMATION", envx.VarBool(true)).
		Environ()
}

// attempt to build the terraform environment that properly
func Env() []string {
	return errorsx.Must(env())
}

// Create a shell runtime that properly
// sets up the yarn environment for caching.
func Runtime() shell.Command {
	return shell.Runtime().EnvironFrom(Env()...)
}
