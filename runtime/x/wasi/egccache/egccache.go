package egccache

import (
	"context"
	"fmt"
	"path/filepath"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func CacheDirectory(dirs ...string) string {
	return egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "ccache", filepath.Join(dirs...))
}

type option = func(*envx.Builder)
type options []option

func Options() options {
	return options(nil)
}

func (t options) Env() []string {
	return errorsx.Must(t.env())
}

func (t options) env() ([]string, error) {
	return langx.Clone(langx.Autoderef(
		envx.Build().Var(
			"CCACHE_DIR", CacheDirectory(),
		),
	), t...).Environ()
}

func (t options) Log(path string) options {
	return append(t, func(b *envx.Builder) {
		b.Var("CCACHE_LOGFILE", path)
	})
}

func (t options) MaxDisk(n uint64) options {
	return append(t, func(b *envx.Builder) {
		b.Var("CCACHE_MAXSIZE", fmt.Sprintf("%Xs", bytesx.Unit(n)))
	})
}

// temporarily disabling ccache. When set, ccache will
// call the real compiler and bypass the cache entirely
func (t options) Disable() options {
	return append(t, func(b *envx.Builder) {
		b.Var("CCACHE_DISABLE", "1")
	})
}

// When this is set, ccache will not use any existing cached results.
// Instead, it will recompile the code and update the cache with the new results.
// This is useful if you suspect a cache corruption.
func (t options) Recache() options {
	return append(t, func(b *envx.Builder) {
		b.Var("CCACHE_RECACHE=1", "1")
	})
}

// attempt to build the ccache environment that sets up
// the ccache environment for caching.
func Env() []string {
	return Options().Env()
}

// Create a shell runtime that properly
// sets up the ccache environment for caching.
func Runtime() shell.Command {
	return shell.Runtime().EnvironFrom(Env()...)
}

// print informational statistics about cache usage
func PrintStatistics(ctx context.Context, o eg.Op) error {
	return shell.Run(ctx, shell.New("ccache -sv"))
}
