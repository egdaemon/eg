package interp

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiegcontainer"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiexec"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigit"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigraph"
	"github.com/egdaemon/eg/runners"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"google.golang.org/grpc"
)

type Option func(*runner)

// OptionModuleDir name of the directory that contains eg directives
func OptionModuleDir(s string) Option {
	return func(r *runner) {
		r.moduledir = s
	}
}

// OptionRuntimeDir
func OptionRuntimeDir(s string) Option {
	return func(r *runner) {
		r.runtimedir = s
	}
}

type runtimefn func(r runner, moduledir string, cmdenv []string, host wazero.HostModuleBuilder) wazero.HostModuleBuilder

// Remote uses the api to implement particular actions like building and running containers.
// may not be necessary.
func Remote(ctx context.Context, runid string, g ffigraph.Eventer, svc grpc.ClientConnInterface, dir string, module string, options ...Option) error {
	var (
		r = runner{
			root:       dir,
			moduledir:  ".eg",
			runtimedir: runners.DefaultRunnerDirectory(runid),
			initonce:   &sync.Once{},
		}
	)

	for _, opt := range options {
		opt(&r)
	}

	containers := c8s.NewProxyClient(svc)

	runtimeenv := func(r runner, moduledir string, cmdenv []string, host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
		return host.NewFunctionBuilder().WithFunc(ffigraph.Analysing(false)).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Analysing").
			NewFunctionBuilder().WithFunc(g.Pusher()).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Push").
			NewFunctionBuilder().WithFunc(g.Popper()).Export("github.com/egdaemon/eg/runtime/wasi/runtime/graph.Pop").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Pull(func(ctx context.Context, name, wdir string, options ...string) (err error) {
			_, err = containers.Pull(ctx, &c8s.PullRequest{
				Name:    name,
				Dir:     wdir,
				Options: options,
			})
			if err != nil {
				return err
			}
			return nil
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Pull").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Build(func(ctx context.Context, name, wdir string, definition string, options ...string) (err error) {
			_, err = containers.Build(ctx, &c8s.BuildRequest{
				Name:       name,
				Directory:  wdir,
				Definition: definition,
				Options:    options,
			})
			if err != nil {
				return err
			}
			return nil
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Build").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Module(func(ctx context.Context, name, modulepath string, options ...string) (err error) {
			cname := fmt.Sprintf("%s.%s", name, md5x.String(modulepath+runid))
			options = append(
				options,
				"--volume", fmt.Sprintf("%s:/opt/egmodule.wasm:ro", filepath.Join(r.moduledir, modulepath)),
			)

			_, err = containers.Module(ctx, &c8s.ModuleRequest{
				Image:   name,
				Name:    cname,
				Mdir:    r.moduledir,
				Options: options,
			})
			if err != nil {
				return err
			}
			return nil
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Module").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Run(func(ctx context.Context, name, modulepath string, cmd []string, options ...string) (err error) {
			cname := fmt.Sprintf("%s.%s", name, md5x.String(modulepath+runid))
			_, err = containers.Run(ctx, &c8s.RunRequest{
				Image:   name,
				Name:    cname,
				Command: cmd,
				Options: options,
			})
			if err != nil {
				return err
			}
			return nil
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Run").
			NewFunctionBuilder().WithFunc(ffiexec.Exec(func(cmd *exec.Cmd) *exec.Cmd {
			cmd.Dir = filepath.Join(r.root, cmd.Dir)
			cmd.Env = append(cmdenv, cmd.Env...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd
		})).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiexec.Command").
			NewFunctionBuilder().WithFunc(
			ffigit.Commitish(r.root),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Commitish").
			NewFunctionBuilder().WithFunc(
			ffigit.Clone(r.root),
		).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Clone")
	}

	return r.perform(ctx, runid, module, runtimeenv)
}

type runner struct {
	root       string
	runtimedir string
	moduledir  string
	initonce   *sync.Once
}

func (t runner) Open(name string) (fs.File, error) {
	path := filepath.Join(t.root, filepath.Clean(name))

	if f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600); err == nil {
		return f, nil
	} else if errors.Is(err, syscall.EISDIR) {
		return os.OpenFile(path, os.O_RDONLY, 0600)
	} else {
		return nil, err
	}
}

func (t runner) perform(ctx context.Context, runid, path string, rtb runtimefn) (err error) {
	moduledir := filepath.Join(t.root, t.moduledir)
	hostcachedir := filepath.Join(moduledir, ".cache")
	guestcachedir := filepath.Join("/", "cache")
	guestruntimedir := runners.DefaultRunnerRuntimeDir()
	tmpdir, err := os.MkdirTemp(t.root, "eg.tmp.*")
	if err != nil {
		return errorsx.Wrap(err, "unable to create tmp directory")
	}
	defer os.RemoveAll(tmpdir)

	cache, err := wazero.NewCompilationCacheWithDir(hostcachedir)
	if err != nil {
		return err
	}
	defer cache.Close(ctx)

	// Create a new WebAssembly Runtime.
	runtime := wazero.NewRuntimeWithConfig(
		ctx,
		wazero.NewRuntimeConfig().WithCompilationCache(cache),
	)

	cmdenv := append(
		os.Environ(),
		fmt.Sprintf("CI=%t", envx.Boolean(false, "EG_CI", "CI")),
		fmt.Sprintf("EG_CI=%t", envx.Boolean(false, "EG_CI", "CI")),
		fmt.Sprintf("EG_RUN_ID=%s", runid),
		fmt.Sprintf("EG_ROOT_DIRECTORY=%s", t.root),
		fmt.Sprintf("EG_CACHE_DIRECTORY=%s", envx.String(guestcachedir, "EG_CACHE_DIRECTORY", "CACHE_DIRECTORY")),
		fmt.Sprintf("EG_RUNTIME_DIRECTORY=%s", guestruntimedir),
		fmt.Sprintf("RUNTIME_DIRECTORY=%s", guestruntimedir),
	)

	log.Println("module dir", moduledir)
	log.Println("cache dir", hostcachedir, "->", guestcachedir)
	log.Println("runtime dir", t.runtimedir, "->", guestruntimedir)

	mcfg := wazero.NewModuleConfig().WithEnv(
		"CI", envx.String("false", "EG_CI", "CI"),
	).WithEnv(
		"EG_CI", envx.String("false", "EG_CI", "CI"),
	).WithEnv(
		"EG_RUN_ID", runid,
	).WithEnv(
		"EG_ROOT_DIRECTORY", t.root,
	).WithEnv(
		"EG_CACHE_DIRECTORY", envx.String(guestcachedir, "EG_CACHE_DIRECTORY", "CACHE_DIRECTORY"),
	).WithEnv(
		"EG_RUNTIME_DIRECTORY", tmpdir,
	).WithEnv(
		"RUNTIME_DIRECTORY", guestruntimedir,
	).WithEnv(
		"HOME", userx.HomeDirectoryOrDefault("/root"),
	).WithEnv(
		"TERM", envx.String("", "TERM"),
	).WithStdin(
		os.Stdin,
	).WithStderr(
		os.Stderr,
	).WithStdout(
		os.Stdout,
	).WithFSConfig(
		wazero.NewFSConfig().
			WithDirMount(hostcachedir, guestcachedir).
			WithDirMount(tmpdir, "/tmp").
			WithDirMount(t.runtimedir, guestruntimedir),
	).WithSysNanotime().WithSysWalltime().WithRandSource(rand.Reader)

	environ := errorsx.Zero(envx.FromPath("/opt/egruntime/environ.env"))
	// envx.Debug(environ...)
	mcfg = wasix.Environ(mcfg, environ...)

	wasienv, err := wasi_snapshot_preview1.NewBuilder(runtime).Instantiate(ctx)
	if err != nil {
		return err
	}
	defer wasienv.Close(ctx)

	hostenv, err := rtb(t, moduledir, cmdenv, runtime.NewHostModuleBuilder("env")).
		Instantiate(ctx)
	if err != nil {
		return err
	}
	defer hostenv.Close(ctx)

	// wasidebug.Host(hostenv)
	log.Println("interp initiated", path)
	defer log.Println("interp completed", path)
	wasi, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	c, err := runtime.CompileModule(ctx, wasi)
	if err != nil {
		return err
	}
	defer c.Close(ctx)

	// wasidebug.Module(c)

	m, err := runtime.InstantiateModule(ctx, c, mcfg.WithName(path))
	if err != nil {
		return err
	}
	defer m.Close(ctx)

	return nil
}
