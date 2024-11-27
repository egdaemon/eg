package egyarn

import (
	"os"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func CacheDirectory() string {
	return egenv.CacheDirectory("egyarn")
}

// attempt to build the yarn environment that properly
func Env() ([]string, error) {
	return envx.Build().FromEnv(os.Environ()...).
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
