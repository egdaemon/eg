package interp

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/interp/runtime/wasi/exechost"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Option func(*runner)

// OptionModuleDir name of the directory that contains eg directives
func OptionModuleDir(s string) Option {
	return func(r *runner) {
		r.moduledir = s
	}
}

func OptionBuildDir(s string) Option {
	return func(r *runner) {
		r.builddir = s
	}
}

func Run(ctx context.Context, dir string, options ...Option) error {
	var (
		r = runner{
			root:      dir,
			moduledir: ".eg",
			builddir:  filepath.Join(".cache", "build"),
		}
	)

	for _, opt := range options {
		opt(&r)
	}

	return r.perform(ctx)
}

type runner struct {
	root      string
	moduledir string
	builddir  string
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

func (t runner) perform(ctx context.Context) (err error) {
	// Create a new WebAssembly Runtime.
	runtime := wazero.NewRuntimeWithConfig(
		ctx,
		wazero.NewRuntimeConfig(),
	)

	moduledir := filepath.Join(t.root, t.moduledir)
	cachedir := filepath.Join(t.root, t.moduledir, ".cache")
	tmpdir, err := os.MkdirTemp(moduledir, "eg.tmp.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	mcfg := wazero.NewModuleConfig().WithEnv(
		"CI", os.Getenv("CI"),
	).WithEnv(
		"EG_CI", os.Getenv("EG_CI"),
	).WithEnv(
		"EG_CACHE_DIRECTORY", envx.String(cachedir, "EG_CACHE_DIRECTORY", "CACHE_DIRECTORY"),
	).WithEnv(
		"EG_RUNTIME_DIRECTORY", tmpdir,
	).WithEnv(
		"RUNTIME_DIRECTORY", tmpdir,
	).WithStderr(
		os.Stderr,
	).WithStdout(
		os.Stdout,
	).WithFS(
		// t,
		os.DirFS("."),
	).WithSysNanotime().WithSysWalltime()

	ns1 := runtime.NewNamespace(ctx)

	wasienv, err := wasi_snapshot_preview1.NewBuilder(runtime).Instantiate(ctx, ns1)
	if err != nil {
		return err
	}
	defer wasienv.Close(ctx)

	hostenv, err := runtime.NewHostModuleBuilder("env").
		NewFunctionBuilder().WithFunc(exechost.Exec(func(cmd *exec.Cmd) *exec.Cmd {
		cmd.Dir = t.root
		cmd.Env = os.Environ()
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		return cmd
	})).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/ffiexec.Command").
		Instantiate(ctx, ns1)
	if err != nil {
		return err
	}
	defer hostenv.Close(ctx)

	// debugmodule2("env", hostenv)

	err = fs.WalkDir(os.DirFS(t.root), t.builddir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			return nil
		}

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

		// debugmodule1(path, c)

		m, err := ns1.InstantiateModule(
			ctx,
			c,
			mcfg.WithName(path),
		)
		if err != nil {
			return err
		}
		defer m.Close(ctx)

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

type debuggable1 interface {
	Name() string
	ExportedFunctions() map[string]api.FunctionDefinition
	ImportedFunctions() []api.FunctionDefinition
}

type debuggable2 interface {
	Name() string
	ExportedFunctionDefinitions() map[string]api.FunctionDefinition
}

func debugmodule1(name string, m debuggable1) {
	log.Println("module debug", name, m.Name())
	for _, imp := range m.ExportedFunctions() {
		paramtypestr := typeliststr(imp.ParamTypes()...)
		resulttypestr := typeliststr(imp.ResultTypes()...)
		log.Println("exported", imp.Name(), "(", paramtypestr, ")", resulttypestr)
	}

	for _, imp := range m.ImportedFunctions() {
		paramtypestr := typeliststr(imp.ParamTypes()...)
		resulttypestr := typeliststr(imp.ResultTypes()...)
		log.Println("imported", imp.Name(), "(", paramtypestr, ")", resulttypestr)
	}
}

func debugmodule2(name string, m debuggable2) {
	log.Println("module debug", name, m.Name())
	for _, imp := range m.ExportedFunctionDefinitions() {
		paramtypestr := typeliststr(imp.ParamTypes()...)
		resulttypestr := typeliststr(imp.ResultTypes()...)
		log.Println("exported", imp.Name(), "(", paramtypestr, ")", resulttypestr)
	}
}

func typeliststr(types ...api.ValueType) string {
	typesstr := []string(nil)
	for _, t := range types {
		typesstr = append(typesstr, api.ValueTypeName(t))
	}

	return strings.Join(typesstr, ", ")
}
