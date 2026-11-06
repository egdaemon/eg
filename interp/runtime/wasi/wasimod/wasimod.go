// Package wasimod loads a directory of WASI modules into the guest's runtime. each *.wasi file is instantiated
// under its file name without running its start functions, so a guest binds to its exports with go:wasmimport.
// an empty or missing directory loads nothing.
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
// is instantiated under its own name from the module of the file it points to, which is compiled once.
func Load(ctx context.Context, runtime wazero.Runtime, dir string, cfg wazero.ModuleConfig) (loaded Modules, err error) {
	var (
		paths    []string
		target   string
		bin      []byte
		mod      api.Module
		compiled = make(map[string]wazero.CompiledModule)
	)

	if paths, err = filepath.Glob(filepath.Join(dir, "*.wasi")); err != nil {
		return nil, err
	}

	for _, path := range paths {
		if target, err = filepath.EvalSymlinks(path); err != nil {
			loaded.Close(ctx)
			return nil, errorsx.Wrapf(err, "unable to resolve module %s", path)
		}

		if _, ok := compiled[target]; !ok {
			if bin, err = os.ReadFile(target); err != nil {
				loaded.Close(ctx)
				return nil, errorsx.Wrapf(err, "unable to read module %s", target)
			}

			if compiled[target], err = runtime.CompileModule(ctx, bin); err != nil {
				loaded.Close(ctx)
				return nil, errorsx.Wrapf(err, "unable to compile module %s", target)
			}
		}

		if mod, err = runtime.InstantiateModule(ctx, compiled[target], cfg.WithName(strings.TrimSuffix(filepath.Base(path), ".wasi")).WithStartFunctions()); err != nil {
			loaded.Close(ctx)
			return nil, errorsx.Wrapf(err, "unable to instantiate module %s", path)
		}

		loaded = append(loaded, mod)
	}

	return loaded, nil
}
