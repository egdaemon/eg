package egccache

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
	return egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "ccache", filepath.Join(dirs...))
}

// attempt to build the ccache environment that sets up
// the cargo environment for caching.
func env() ([]string, error) {
	return envx.Build().FromEnv(os.Environ()...).
		Var("CCACHE_DIR", CacheDirectory()).
		Environ()
}

// attempt to build the ccache environment that sets up
// the ccache environment for caching.
func Env() []string {
	return errorsx.Must(env())
}

// Create a shell runtime that properly
// sets up the ccache environment for caching.
func Runtime() shell.Command {
	return shell.Runtime().EnvironFrom(Env()...)
}
