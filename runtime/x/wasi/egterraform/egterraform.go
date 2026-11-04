// Package egterraform has supporting functions for configuring the environment for running terraform commands
// within eg.
package egterraform

import (
	"path/filepath"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func CacheDirectory(dirs ...string) string {
	return egenv.DirectoryCache(_eg.DefaultModuleDirectory(), "terraform", filepath.Join(dirs...))
}

// attempt to build the terraform environment that properly
func env() ([]string, error) {
	return envx.Build().
		Var("TF_PLUGIN_CACHE_DIR", CacheDirectory("plugins")).
		Var("TF_IN_AUTOMATION", envx.VarBool(true)).
		Environ()
}

// attempt to build the terraform environment that properly
func Env() []string {
	return errorsx.Must(env())
}
