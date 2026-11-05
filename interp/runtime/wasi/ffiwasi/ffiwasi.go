// Package ffiwasi runs commands from a directory of WASI modules inside the guest's runtime. every *.wasi file
// in the directory is compiled into the runtime and runs the command its name gives, so an empty or missing
// directory adds nothing.
package ffiwasi

import (
	"context"
	"crypto/rand"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/interp/execproxy/v2/proc"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"
)

// Directory the guest's runtime loads its modules from.
func Directory() string {
	return userx.DefaultDirLocation("wasi.mod.d")
}

// Set is the modules compiled into a runtime, by the command each runs.
type Set struct {
	runtime    wazero.Runtime
	mounts     wazero.FSConfig
	workdir    string
	runtimedir string
	modules    map[string]wazero.CompiledModule
	exits      sync.Map
}

// Load compiles the *.wasi modules in dir into runtime. a link shares the module of the file it points to, so
// one multicall module serves every name linked to it. the commands see mounts, the guest's filesystem, with
// their directory, relative to workdir, mounted at . and their stdio on the guest's sockets in runtimedir.
func Load(ctx context.Context, runtime wazero.Runtime, dir string, mounts wazero.FSConfig, workdir string, runtimedir string) (s *Set, err error) {
	var (
		paths    []string
		target   string
		bin      []byte
		compiled = make(map[string]wazero.CompiledModule)
	)

	s = &Set{runtime: runtime, mounts: mounts, workdir: workdir, runtimedir: runtimedir, modules: make(map[string]wazero.CompiledModule)}

	if paths, err = filepath.Glob(filepath.Join(dir, "*.wasi")); err != nil {
		return nil, err
	}

	for _, path := range paths {
		if target, err = filepath.EvalSymlinks(path); err != nil {
			return nil, errorsx.Wrapf(err, "unable to resolve module %s", path)
		}

		if _, ok := compiled[target]; !ok {
			if bin, err = os.ReadFile(target); err != nil {
				return nil, errorsx.Wrapf(err, "unable to read module %s", target)
			}

			if compiled[target], err = runtime.CompileModule(ctx, bin); err != nil {
				return nil, errorsx.Wrapf(err, "unable to compile module %s", target)
			}
		}

		s.modules[strings.TrimSuffix(filepath.Base(path), ".wasi")] = compiled[target]
	}

	return s, nil
}

// Export adds the host functions that start and wait on the set's commands to the guest's host module.
func (t *Set) Export(host wazero.HostModuleBuilder) wazero.HostModuleBuilder {
	return host.
		NewFunctionBuilder().WithFunc(t.start).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiwasi.Start").
		NewFunctionBuilder().WithFunc(t.wait).Export("github.com/egdaemon/eg/runtime/wasi/runtime/ffiwasi.Wait")
}

// start runs the command in its module, in the background: the guest is a single thread, so it can only copy
// the command's stdio once the call returns. it returns ffierrors.ErrNotFound for a command the set has no
// module for.
func (t *Set) start(
	ctx context.Context,
	m api.Module,
	idoffset uint32, idlen uint32,
	diroffset uint32, dirlen uint32,
	envoffset uint32, envlen uint32, envsize uint32,
	nameoffset uint32, namelen uint32,
	argsoffset uint32, argslen uint32, argssize uint32,
) uint32 {
	var (
		err      error
		id       string
		dir      string
		name     string
		args     []string
		environ  []string
		stdio    []*os.File
		compiled wazero.CompiledModule
		known    bool
	)

	if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
		log.Println("unable to read the command name", err)
		return ffierrors.ErrUnrecoverable
	}

	if compiled, known = t.modules[name]; !known {
		return ffierrors.ErrNotFound
	}

	if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
		log.Println("unable to read the process id", err)
		return ffierrors.ErrUnrecoverable
	}

	if dir, err = ffi.ReadString(m.Memory(), diroffset, dirlen); err != nil {
		log.Println("unable to read the command directory", err)
		return ffierrors.ErrUnrecoverable
	}

	if args, err = ffi.ReadStringArray(m.Memory(), argsoffset, argslen, argssize); err != nil {
		log.Println("unable to read the command arguments", err)
		return ffierrors.ErrUnrecoverable
	}

	if environ, err = ffi.ReadStringArray(m.Memory(), envoffset, envlen, envsize); err != nil {
		log.Println("unable to read the command environment", err)
		return ffierrors.ErrUnrecoverable
	}

	if stdio, err = proc.Dial(t.runtimedir, id); err != nil {
		log.Println(err)
		return ffierrors.ErrUnrecoverable
	}

	if !filepath.IsAbs(dir) {
		dir = filepath.Join(t.workdir, dir)
	}

	cfg := wazero.NewModuleConfig().
		WithName("").
		WithArgs(append([]string{name}, args...)...).
		WithStdin(stdio[0]).
		WithStdout(stdio[1]).
		WithStderr(stdio[2]).
		// the module has no working directory, so relative paths resolve within the one mounted at . wazero
		// treats . as /, so the command's directory also takes the place of the guest's empty root.
		WithFSConfig(t.mounts.WithDirMount(dir, ".")).
		WithSysWalltime().
		WithSysNanotime().
		WithRandSource(rand.Reader)

	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok {
			cfg = cfg.WithEnv(k, v)
		}
	}

	exit := make(chan uint32, 1)
	t.exits.Store(id, exit)

	go func() {
		defer func() {
			for _, f := range stdio {
				f.Close()
			}
		}()

		exit <- t.run(ctx, compiled, cfg)
	}()

	return 0
}

// run instantiates the module and returns its exit code.
func (t *Set) run(ctx context.Context, compiled wazero.CompiledModule, cfg wazero.ModuleConfig) uint32 {
	var (
		err error
		mod api.Module
	)

	if mod, err = t.runtime.InstantiateModule(ctx, compiled, cfg); mod != nil {
		mod.Close(ctx)
	}

	if exit, ok := errors.AsType[*sys.ExitError](err); ok {
		return exit.ExitCode()
	}

	if err != nil {
		log.Println("unable to run the module", err)
		return 127
	}

	return 0
}

// wait returns the exit code of the command started as the process id, and ffierrors.ErrPending while it runs.
func (t *Set) wait(ctx context.Context, m api.Module, idoffset uint32, idlen uint32) uint32 {
	var (
		err  error
		id   string
		exit any
		ok   bool
	)

	if id, err = ffi.ReadString(m.Memory(), idoffset, idlen); err != nil {
		log.Println("unable to read the process id", err)
		return ffierrors.ErrUnrecoverable
	}

	if exit, ok = t.exits.Load(id); !ok {
		return ffierrors.ErrNotFound
	}

	select {
	case code := <-exit.(chan uint32):
		t.exits.Delete(id)
		return code
	default:
		return ffierrors.ErrPending
	}
}
