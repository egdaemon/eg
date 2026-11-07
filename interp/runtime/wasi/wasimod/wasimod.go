// Package wasimod loads a directory of WASI modules into the guest's runtime. each *.wasi file is instantiated
// under its file name, so a guest binds to its exports with go:wasmimport. a reactor module's _initialize runs
// during instantiation, as a reactor requires before its exports are called. a command module's _start never runs,
// as it would exit the module. an empty or missing directory loads nothing.
package wasimod

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Directory the guest's runtime loads its modules from.
func Directory() string {
	return userx.DefaultDirLocation("wasi.mod.d")
}

// Modules loaded into a runtime.
type Modules []api.Module

// Close the modules.
func (t Modules) Close(ctx context.Context) {
	for _, m := range t {
		m.Close(ctx)
	}
}

// Load compiles the *.wasi modules in dir and instantiates each in runtime with cfg, under its file name. a link
// is instantiated under its own name from the module of the file it points to. only _initialize is run at
// instantiation, wazero skips it for modules that do not export it, i.e. commands.
func Load(ctx context.Context, runtime wazero.Runtime, dir string, cfg wazero.ModuleConfig) (loaded Modules, err error) {
	var (
		paths    []string
		bin      []byte
		compiled wazero.CompiledModule
		mod      api.Module
	)

	if paths, err = filepath.Glob(filepath.Join(dir, "*.wasi")); err != nil {
		return nil, err
	}

	for _, path := range paths {
		if bin, err = os.ReadFile(path); err != nil {
			loaded.Close(ctx)
			return nil, errorsx.Wrapf(err, "unable to read module %s", path)
		}

		if compiled, err = runtime.CompileModule(ctx, bin); err != nil {
			loaded.Close(ctx)
			return nil, errorsx.Wrapf(err, "unable to compile module %s", path)
		}

		mod, err = runtime.InstantiateModule(ctx, compiled, cfg.WithName(strings.TrimSuffix(filepath.Base(path), ".wasi")).WithStartFunctions("_initialize"))
		// the instance keeps its machine code alive, so the compiled module is closed right away. this drops it
		// from the runtime's in memory cache: another link to the same file loads its own copy from the disk cache.
		compiled.Close(ctx)
		if err != nil {
			loaded.Close(ctx)
			return nil, errorsx.Wrapf(err, "unable to instantiate module %s", path)
		}

		loaded = append(loaded, mod)
	}

	return loaded, nil
}
