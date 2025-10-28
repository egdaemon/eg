package egollama

import (
	"context"
	"errors"
	"path/filepath"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func CacheDirectory(dirs ...string) string {
	return egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "ollama", filepath.Join(dirs...))
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
			"OLLAMA_MODELS", CacheDirectory("models"),
		),
	), t...).Environ()
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

// pull a specific model
func Pull(runtime shell.Command, model string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return shell.Run(ctx, runtime.Newf("ollama pull %s", model))
	}
}

func Serve(runtime shell.Command) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return shell.Run(ctx, runtime.Newf("systemd-run --user --unit=ollama.service ollama serve"))
	}
}

func Shutdown(runtime shell.Command) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return shell.Run(ctx, runtime.Newf("systemctl stop --user ollama.service"))
	}
}

// run the provided operation with the given model.
func With(model string, op eg.OpFn) eg.OpFn {
	rt := Runtime()
	return func(ctx context.Context, o eg.Op) error {
		return around(
			Serve(rt),
			eg.Sequential(
				Pull(rt, model),
				op,
			),
			Shutdown(rt),
		)(ctx, o)
	}
}

// run an operation before and after another operation.
// the after operation *always* runs after the middle operation.
// but not if the before op fails.
func around(before eg.OpFn, op eg.OpFn, after eg.OpFn) eg.OpFn {
	return func(octx context.Context, o eg.Op) (err error) {
		if err = before(octx, o); err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, after(octx, o))
		}()
		return op(octx, o)
	}
}
