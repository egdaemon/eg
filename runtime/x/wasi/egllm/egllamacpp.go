package egllm

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
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
	"golang.org/x/time/rate"
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
// records which model and cache the systemd unit should use, since the unit
// does not inherit the runtime's environment.
func Pull(runtime shell.Command, model string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		log.Printf("egllm: warming model cache; llama-completion output follows\n")
		return shell.Run(
			ctx,
			// llama-completion shares llama-server's -hf cache layout; running it
			// with a minimal prediction count downloads the model without
			// needing to stand up the HTTP server.
			runtime.Newf("llama-completion -hf %s -no-cnv -n 1 -p ok", model),
			runtime.Newf("printf 'LLAMA_MODEL=%%s\\nLLAMA_CACHE=%%s\\n' %s %s > /etc/default/llama-server", model, CacheDirectory("models")).Privileged(),
		)
	}
}

// Install copies the embedded llama-server systemd unit into place, replacing any unit already
// present, so the image doesn't need to provide one. run it before Serve.
func Install(runtime shell.Command) eg.OpFn {
	const unit = "llama-server.service"

	return func(ctx context.Context, o eg.Op) error {
		if err := egfs.CloneFS(ctx, egenv.EphemeralDirectory(), unit, skel); err != nil {
			return err
		}

		return shell.Run(
			ctx,
			runtime.Newf("install -m 0644 %s /etc/systemd/system/%s", egenv.EphemeralDirectory(unit), unit).Privileged(),
			runtime.New("systemctl daemon-reload").Privileged(),
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
// loading into memory yet. large models can take minutes to load.
func waitHealthy(runtime shell.Command, timeout time.Duration) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		target := fmt.Sprintf("%s/health", Endpoint())
		var curl string
		if path, ok := socketPath(Endpoint()); ok {
			target, curl = "http://localhost/health", fmt.Sprintf("--unix-socket %s", path)
		}

		if path, ok := socketPath(Endpoint()); ok {
			debugSocket(ctx, runtime, path)
		}

		// polling is paced by the runtime's retry limiter, see shell.Command.Retry.
		ctx, done := context.WithTimeout(ctx, timeout)
		defer done()

		return shell.Run(ctx, runtime.Newf(
			"curl -sf --max-time 5 %s %s > /dev/null",
			curl,
			target,
		).Attempts(-1))
	}
}

// debugUnit logs the installed llama-server unit, the checksum of the file and the unit as systemd loaded it,
// so the flags the server will start with are known before it is started. failures are ignored, this is purely informational.
func debugUnit(runtime shell.Command) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		for _, cmd := range []string{
			"md5sum /etc/systemd/system/llama-server.service",
			"systemctl cat llama-server.service --no-pager",
			"systemctl show llama-server.service --property=ActiveState,SubState,FragmentPath,ExecStart --no-pager",
		} {
			if err := shell.Run(ctx, runtime.Newf("%s", cmd).Lenient(true).Attempts(1)); err != nil {
				log.Printf("egllm: debug command failed: %s: %v\n", cmd, err)
			}
		}

		return nil
	}
}

// debugSocket logs the state of the llama-server unix socket and its service before polling it.
// failures are ignored, this is purely informational.
func debugSocket(ctx context.Context, runtime shell.Command, path string) {
	dir := filepath.Dir(path)
	log.Printf("egllm: debugging unix socket %s\n", path)
	for _, cmd := range []string{
		fmt.Sprintf("id; ls -lad %s %s", dir, path),
		fmt.Sprintf("ls -la %s", dir),
		"systemctl cat llama-server.service --no-pager",
		"systemctl status llama-server.service --no-pager -l",
		"journalctl -u llama-server.service -n 20 --no-pager",
	} {
		if err := shell.Run(ctx, runtime.Newf("%s", cmd).Lenient(true).Attempts(1)); err != nil {
			log.Printf("egllm: debug command failed: %s: %v\n", cmd, err)
		}
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

// run the provided operation with the given model. a llama-server is started for the
// duration of the operation, unless the endpoint has been overridden (see eg.EnvComputeLLMEndpoint)
// in which case the operation is run against that server as is.
func With(model string, op eg.OpFn) eg.OpFn {
	if overridden() {
		return external(Endpoint(), op)
	}

	return managed(model, op)
}

// external runs op against a server that isn't ours to manage.
func external(endpoint string, op eg.OpFn) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		log.Printf("egllm: using external endpoint %s, skipping server setup\n", endpoint)
		return phase("operation", op)(ctx, o)
	}
}

func managed(model string, op eg.OpFn) eg.OpFn {
	// the limiter paces the health polling, the other commands are not retried.
	rt := Runtime().Retry(rate.NewLimiter(rate.Every(5*time.Second), 1))
	return func(ctx context.Context, o eg.Op) error {
		return around(
			eg.Sequential(
				phase("installing llama-server.service", Install(rt)),
				phase("debugging llama-server.service", debugUnit(rt)),
				phase(fmt.Sprintf("pulling model %s (cache %s)", model, CacheDirectory("models")), Pull(rt, model)),
				phase("starting llama-server.service", Serve(rt)),
				phase(fmt.Sprintf("waiting for llama-server health at %s (timeout 10m)", Endpoint()), waitHealthy(rt, 10*time.Minute)),
			),
			phase("operation", op),
			phase("stopping llama-server.service", Shutdown(rt)),
		)(ctx, o)
	}
}

// phase logs the start and completion, with the elapsed time, of an operation.
func phase(name string, op eg.OpFn) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		log.Printf("egllm: %s: started\n", name)
		start := time.Now()
		err := op(ctx, o)
		log.Printf("egllm: %s: completed in %v err=%v\n", name, time.Since(start), err)
		return err
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
