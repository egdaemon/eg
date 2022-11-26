package interp

import (
	"context"
	"io/fs"
	"log"
	"os"
	"strings"

	"github.com/james-lawrence/eg/internal/osx"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Option func(*runner)

func Run(ctx context.Context, dir string, options ...Option) error {
	var (
		r = runner{
			root: dir,
		}
	)

	for _, opt := range options {
		opt(&r)
	}

	log.Println("loading", dir)
	return r.perform(ctx)
}

type runner struct {
	root string
}

func (t runner) perform(ctx context.Context) (err error) {
	// Create a new WebAssembly Runtime.
	runtime := wazero.NewRuntimeWithConfig(
		ctx,
		wazero.NewRuntimeConfig(),
	)

	mcfg := wazero.NewModuleConfig().WithEnv(
		"CI", os.Getenv("CI"),
	).WithEnv(
		"EG_CI", os.Getenv("EG_CI"),
	).WithStderr(
		os.Stderr,
	).WithStdout(
		os.Stdout,
	).WithSysNanotime()

	log.Println("DERP", os.Getenv("CI"))

	ns1 := runtime.NewNamespace(ctx)

	wasienv, err := wasi_snapshot_preview1.NewBuilder(runtime).Instantiate(ctx, ns1)
	if err != nil {
		return err
	}
	defer wasienv.Close(ctx)

	// hostenv, err := runtime.NewHostModuleBuilder("env").Instantiate(ctx, ns1)
	// if err != nil {
	// 	return err
	// }
	// defer hostenv.Close(ctx)

	err = fs.WalkDir(os.DirFS(osx.Getwd(".")), t.root, func(path string, d fs.DirEntry, err error) error {
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

		log.Println("compiling initiated", path)
		defer log.Println("compiling completed", path)
		wasi, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		c, err := runtime.CompileModule(ctx, wasi)
		if err != nil {
			return err
		}
		defer c.Close(ctx)

		debugmodule(path, c)

		name := path
		if strings.HasSuffix(path, "runtime.wasm") {
			name = "env"
		}
		_, err = ns1.InstantiateModule(
			ctx,
			c,
			mcfg.WithName(name),
		)
		if err != nil {
			return err
		}
		// defer m.Close(ctx)

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func debugmodule(name string, m wazero.CompiledModule) {
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

func typeliststr(types ...api.ValueType) string {
	typesstr := []string(nil)
	for _, t := range types {
		typesstr = append(typesstr, api.ValueTypeName(t))
	}

	return strings.Join(typesstr, ", ")
}
