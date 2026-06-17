package egllm

import (
	"context"
	"embed"
	"errors"
	"path/filepath"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

//go:embed Containerfile llama-server.service
var skel embed.FS

func CacheDirectory(dirs ...string) string {
	return egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "llama.cpp", filepath.Join(dirs...))
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
			"LLAMA_CACHE", CacheDirectory("models"),
		),
	), t...).Environ()
}

// attempt to build the environment that sets up
// the llama.cpp model cache directory.
func Env() []string {
	return Options().Env()
}

// Create a shell runtime that properly
// sets up the llama.cpp caching environment.
func Runtime() shell.Command {
	return shell.Runtime().EnvironFrom(Env()...).Timeout(20 * time.Minute)
}

// Pull warms the local model cache for the given model so that starting
// llama-server (which loads the model at process start, unlike ollama's
// decoupled daemon+pull model) never blocks on a cold download. It also
// records which model the systemd unit should load.
func Pull(runtime shell.Command, model string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return shell.Run(
			ctx,
			// llama-cli shares llama-server's -hf cache layout; running it
			// with a minimal prediction count downloads the model without
			// needing to stand up the HTTP server.
			runtime.Newf("llama-cli -hf %s -no-cnv -n 1 -p ok", model),
			runtime.Newf("echo LLAMA_MODEL=%s > /etc/default/llama-server", model),
		)
	}
}

func Serve(runtime shell.Command) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return shell.Run(ctx, runtime.Newf("systemctl enable --now llama-server.service").Privileged())
	}
}

func Shutdown(runtime shell.Command) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return shell.Run(ctx, runtime.Newf("systemctl stop llama-server.service").Privileged())
	}
}

// waitHealthy polls llama-server's health endpoint until it responds, since
// systemd reporting the unit "active" doesn't mean the model has finished
// loading into memory yet.
func waitHealthy(runtime shell.Command) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return shell.Run(ctx, runtime.New(
			"for i in $(seq 1 60); do curl -sf http://localhost:8080/health > /dev/null 2>&1 && exit 0; sleep 1; done; exit 1",
		))
	}
}

// build llama.cpp container.
func Prepare(c eg.ContainerRunner) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		const relpath = "Containerfile"
		if err := egfs.CloneFS(ctx, egenv.EphemeralDirectory(), relpath, skel); err != nil {
			return err
		}

		if err := egfs.CloneFS(ctx, egenv.EphemeralDirectory(), "llama-server.service", skel); err != nil {
			return err
		}

		return eg.Build(c.BuildFromFile(filepath.Join(egenv.EphemeralDirectory(), relpath)))(ctx, o)
	}
}

// container for this package.
func Runner() eg.ContainerRunner {
	return eg.Container("eg.llamacpp")
}

// run the provided operation with the given model.
func With(model string, op eg.OpFn) eg.OpFn {
	rt := Runtime()
	return func(ctx context.Context, o eg.Op) error {
		return around(
			eg.Sequential(
				Pull(rt, model),
				Serve(rt),
				waitHealthy(rt),
			),
			op,
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
