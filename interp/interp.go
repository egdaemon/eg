package interp

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/md5x"
	"github.com/james-lawrence/eg/internal/osx"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiegcontainer"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiexec"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffigraph"
	"github.com/james-lawrence/eg/runners"
	"github.com/james-lawrence/eg/runtime/wasi/langx"
	"github.com/pkg/errors"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Option func(*runner)

// OptionModuleDir name of the directory that contains eg directives
func OptionModuleDir(s string) Option {
	return func(r *runner) {
		r.moduledir = s
	}
}

type runtimefn func(r runner, moduledir string, cmdenv []string, host wazero.HostModuleBuilder) wazero.HostModuleBuilder

func Run(ctx context.Context, runid string, g ffigraph.Eventer, dir string, module string, options ...Option) error {
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

	stdin := os.Stdin
	stdout := os.Stdout

	runtimeenv := func(r runner, moduledir string, cmdenv []string, host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
		return host.NewFunctionBuilder().WithFunc(ffigraph.Analysing(false)).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Analysing").
			NewFunctionBuilder().WithFunc(g.Pusher()).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Push").
			NewFunctionBuilder().WithFunc(g.Popper()).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Pop").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Pull(func(ctx context.Context, name string, options ...string) (cmd *exec.Cmd, err error) {
			cmd, err = ffiegcontainer.PodmanPull(ctx, name, options...)
			cmd.Dir = r.root
			cmd.Env = cmdenv
			cmd.Stdin = stdin
			cmd.Stdout = stdout
			cmd.Stderr = os.Stderr
			return cmd, err
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Pull").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Build(func(ctx context.Context, name, definition string, options ...string) (cmd *exec.Cmd, err error) {
			cmd, err = ffiegcontainer.PodmanBuild(ctx, name, ".", definition, options...)
			cmd.Dir = r.root
			cmd.Env = cmdenv
			cmd.Stdin = stdin
			cmd.Stdout = stdout
			cmd.Stderr = os.Stderr
			return cmd, err
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Build").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Module(func(ctx context.Context, name, modulepath string, options ...string) (err error) {
			cmdctx := func(cmd *exec.Cmd) *exec.Cmd {
				cmd.Dir = r.root
				cmd.Env = cmdenv
				cmd.Stdin = stdin
				cmd.Stdout = stdout
				cmd.Stderr = os.Stderr
				return cmd
			}
			cname := fmt.Sprintf("%s.%s", name, md5x.DigestString(modulepath+runid))

			options = append(
				options,
				"--volume", fmt.Sprintf("%s:/opt/egbin:ro", langx.Must(exec.LookPath(os.Args[0]))),
				"--volume", fmt.Sprintf("%s:/opt/egmodule.wasm:ro", modulepath),
				"--volume", fmt.Sprintf("%s:/opt/eg:O", r.root),
				"--volume", fmt.Sprintf("%s:/opt/egruntime", r.runtimedir),
			)

			return ffiegcontainer.PodmanModule(ctx, cmdctx, name, cname, r.moduledir, options...)
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Module").
			NewFunctionBuilder().WithFunc(ffiegcontainer.Run(func(ctx context.Context, name, modulepath string, cmd []string, options ...string) (err error) {
			cmdctx := func(cmd *exec.Cmd) *exec.Cmd {
				cmd.Dir = r.root
				cmd.Env = cmdenv
				cmd.Stdin = stdin
				cmd.Stdout = stdout
				cmd.Stderr = os.Stderr
				return cmd
			}
			cname := fmt.Sprintf("%s.%s", name, md5x.DigestString(modulepath+runid))

			options = append(
				options,
				"--volume", fmt.Sprintf("%s:/opt/eg:O", r.root),
			)

			return ffiegcontainer.PodmanRun(ctx, cmdctx, name, cname, cmd, options...)
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Run").
			NewFunctionBuilder().WithFunc(ffiexec.Exec(func(cmd *exec.Cmd) *exec.Cmd {
			cmd.Dir = filepath.Join(r.root, cmd.Dir)
			cmd.Env = cmdenv
			cmd.Stdin = stdin
			cmd.Stdout = stdout
			cmd.Stderr = os.Stderr
			return cmd
		})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiexec.Command")
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
	// Create a new WebAssembly Runtime.
	runtime := wazero.NewRuntimeWithConfig(
		ctx,
		wazero.NewRuntimeConfig(),
	)

	moduledir := filepath.Join(t.root, t.moduledir)
	hostcachedir := filepath.Join(t.root, t.moduledir, ".cache")
	guestcachedir := filepath.Join("/", "cache")
	guestruntimedir := runners.DefaultRunnerRuntimeDir()
	tmpdir, err := os.MkdirTemp(moduledir, "eg.tmp.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	cmdenv := append(
		os.Environ(),
		fmt.Sprintf("CI=%s", envx.String("", "EG_CI", "CI")),
		fmt.Sprintf("EG_CI=%s", envx.String("", "EG_CI", "CI")),
		fmt.Sprintf("EG_RUN_ID=%s", runid),
		fmt.Sprintf("EG_ROOT_DIRECTORY=%s", t.root),
		fmt.Sprintf("EG_CACHE_DIRECTORY=%s", envx.String(guestcachedir, "EG_CACHE_DIRECTORY", "CACHE_DIRECTORY")),
		fmt.Sprintf("EG_RUNTIME_DIRECTORY=%s", guestruntimedir),
		fmt.Sprintf("RUNTIME_DIRECTORY=%s", guestruntimedir),
	)

	log.Println("cache dir", hostcachedir, "->", guestcachedir)
	log.Println("runtime dir", t.runtimedir, "->", guestruntimedir)

	mcfg := wazero.NewModuleConfig().WithEnv(
		"CI", envx.String("", "EG_CI", "CI"),
	).WithEnv(
		"EG_CI", envx.String("", "EG_CI", "CI"),
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
		"HOME", osx.UserHomeDir("/root"),
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
