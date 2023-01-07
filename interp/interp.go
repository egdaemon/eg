package interp

import (
	"context"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strings"

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

func Run(ctx context.Context, dir string, options ...Option) error {
	var (
		r = runner{
			root:      dir,
			moduledir: ".eg",
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
	).WithFS(
		os.DirFS(t.root),
	).WithSysNanotime().WithSysWalltime()

	ns1 := runtime.NewNamespace(ctx)

	wasienv, err := wasi_snapshot_preview1.NewBuilder(runtime).Instantiate(ctx, ns1)
	if err != nil {
		return err
	}
	defer wasienv.Close(ctx)

	hostenv, err := runtime.NewHostModuleBuilder("env").NewFunctionBuilder().WithFunc(func(
		ctx context.Context,
		m api.Module,
		nameoffset uint32, namelen uint32,
		argsoffset uint32, argslen uint32, argssize uint32,
	) uint32 {
		var (
			ok   bool
			data []byte
		)

		if data, ok = m.Memory().Read(nameoffset, namelen); !ok {
			return 127
		}
		name := string(data)

		args := make([]string, 0, argslen)
		for offset, i := argsoffset, uint32(0); i < argslen*argssize; offset, i = offset+8, i+argssize {
			var (
				moffset uint32
				mlen    uint32
			)

			if moffset, ok = m.Memory().ReadUint32Le(argsoffset + i*4); !ok {
				return 127
			}
			if mlen, ok = m.Memory().ReadUint32Le(argsoffset + (i+1)*4); !ok {
				return 127
			}

			if data, ok = m.Memory().Read(moffset, mlen); !ok {
				return 127
			}
			args = append(args, string(data))
		}

		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = t.root
		cmd.Env = os.Environ()
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err = cmd.Run(); err != nil {
			log.Println("failed to execute shell command", err)
			return 128
		}

		return 0
	}).Export("github.com/james-lawrence/eg/runtime/wasi/runtime/host.Command").Instantiate(ctx, ns1)
	if err != nil {
		return err
	}
	defer hostenv.Close(ctx)
	debugmodule2("env", hostenv)

	err = fs.WalkDir(os.DirFS(t.root), t.moduledir, func(path string, d fs.DirEntry, err error) error {
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

		debugmodule1(path, c)

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
