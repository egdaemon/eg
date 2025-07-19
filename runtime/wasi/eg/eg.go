package eg

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiegcontainer"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigraph"
)

// Generally unsafe predefined runner for modules. useful
// for providing a base line environment but has no long term
// stability promises.
func DefaultModule() ContainerRunner {
	path := egunsafe.RuntimeDirectory("eg.default.module")
	errorsx.Never(eg.PrepareRootContainer(path))
	return Container("eg").BuildFromFile(path)
}

// A reference to an operation. used during instrumentation.
type Reference interface {
	ID() string
}

type Op interface {
	ID() string
}

type OpFn func(context.Context, Op) error

type runtimeref struct {
	ptr uintptr
	do  OpFn
}

func (t runtimeref) ID() string {
	return fmt.Sprintf("ref%x", t.ptr)
}

func (t runtimeref) OpInfo(ts time.Time, cause error, path []string) *events.Op {
	fninfo := runtime.FuncForPC(t.ptr)
	file, _ := fninfo.FileLine(t.ptr)
	name := fninfo.Name()

	if strings.HasPrefix(file, "github.com/egdaemon/eg/runtime/wasi/eg") {
		return nil
	}
	return &events.Op{
		State:        events.OpState(cause),
		Milliseconds: int64(time.Since(ts) / time.Millisecond),
		Name:         name,
		Module:       file,
		Path:         path,
	}
}

type namedop string

func (t namedop) ID() string {
	return string(t)
}

func prefixedop(p string, o Op) namedop {
	return namedop(fmt.Sprintf("%s%s", p, o.ID()))
}

// ref meta programming marking a task for delayed execution when rewriting the program at compilation time.
// if executed directly will use the memory location of the function.
// Important: this method acts as an instrumentation point by the runtime.
func ref(o OpFn) Reference {
	addr := reflect.ValueOf(o).Pointer()
	return runtimeref{ptr: addr, do: OpFn(o)}
}

type transpiledref struct {
	name string
	do   OpFn
}

func (t transpiledref) ID() string {
	return t.name
}

func traceOp(op OpFn, r Reference) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return op(ctx, r)
	}
}

// Deprecated: this is intended for internal use only. do not use.
// its use may prevent future builds from executing.
func UnsafeTranspiledRef(name string, o OpFn) Reference {
	return transpiledref{
		name: name,
		do:   o,
	}
}

// execute the provided tasks in sequential order.
func Perform(octx context.Context, operations ...OpFn) error {
	for _, op := range operations {
		r := ref(op)
		if err := ffigraph.TraceErr(octx, r, traceOp(op, r)); err != nil {
			return err
		}
	}

	return nil
}

func Sequential(operations ...OpFn) OpFn {
	return func(octx context.Context, o Op) error {
		parent := prefixedop("seq", o)
		return ffigraph.TraceErr(octx, parent, func(mctx context.Context) error {
			for _, op := range operations {
				r := ref(op)
				if err := ffigraph.TraceErr(mctx, r, traceOp(op, r)); err != nil {
					return err
				}
			}
			return nil
		})
	}
}

// Run operations in parallel.
func Parallel(operations ...OpFn) OpFn {
	return func(octx context.Context, o Op) (err error) {
		parent := prefixedop("par", o)
		errs := make(chan error, len(operations))
		defer close(errs)

		ffigraph.Wrap(octx, parent, func(mctx context.Context) {
			for _, o := range operations {
				go func(iop OpFn) {
					r := ref(iop)
					select {
					case <-octx.Done():
						errs <- octx.Err()
					case errs <- ffigraph.TraceErr(mctx, r, traceOp(iop, r)):
					}
				}(o)
			}
		})

		for i := 0; i < len(operations); i++ {
			select {
			case <-octx.Done():
				return octx.Err()
			case cause := <-errs:
				err = errorsx.Compact(err, cause)
			}
		}

		return err
	}
}

// make a operation conditional on a boolean value.
func When(b bool, o OpFn) OpFn {
	return WhenFn(func(ctx context.Context) bool { return b }, o)
}

// make an operation conditional on a boolean function.
func WhenFn(b func(ctx context.Context) bool, o OpFn) OpFn {
	return func(ctx context.Context, i Op) error {
		if !b(ctx) {
			return nil
		}

		r := ref(o)
		return ffigraph.TraceErr(ctx, r, traceOp(o, r))
	}
}

// interface to workload runners, used to represent containers, vms, or other such runtimes.
type Runner interface {
	CompileWith(ctx context.Context) (err error)
	RunWith(ctx context.Context, mpath string) (err error)
}

// Run the tasks with the specified container.
func Container(name string) ContainerRunner {
	return ContainerRunner{
		name:  name,
		built: &sync.Once{},
	}
}

type coption []string

func (t coption) workdir(dir string) coption {
	return []string{"-w", dir}
}

func (t coption) envvar(k, v string) coption {
	if v == "" {
		return []string{"-e", k}
	}

	return []string{"-e", fmt.Sprintf("%s=%s", k, v)}
}

func (t coption) literal(options ...string) coption {
	return options
}

type ContainerRunner struct {
	name       string
	definition string
	pull       string
	cmd        []string
	options    []coption
	built      *sync.Once
}

func (t ContainerRunner) Clone() ContainerRunner {
	dup := t
	dup.cmd = make([]string, 0, len(t.cmd))
	dup.options = make([]coption, 0, len(t.options))
	copy(dup.cmd, t.cmd)
	copy(dup.options, t.options)
	return t
}

func (t ContainerRunner) OptionLiteral(args ...string) ContainerRunner {
	t.options = append(t.options, coption(nil).literal(args...))
	return t
}

// specifies the location of the container file on disk.
func (t ContainerRunner) BuildFromFile(s string) ContainerRunner {
	t.definition = s
	return t
}

// pull the container from a remote repository.
func (t ContainerRunner) PullFrom(s string) ContainerRunner {
	t.pull = s
	return t
}

// the command to execute.
func (t ContainerRunner) Command(s string) ContainerRunner {
	t.cmd = strings.Split(s, " ")
	return t
}

// CompileWith builds the container and
func (t ContainerRunner) CompileWith(ctx context.Context) (err error) {
	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	t.built.Do(func() {
		if t.pull != "" {
			if err = errorsx.Wrapf(ffiegcontainer.Pull(ctx, t.pull, opts), "unable to pull the container: %s", t.name); err != nil {
				return
			}
		}

		if t.definition != "" {
			if err = errorsx.Wrapf(ffiegcontainer.Build(ctx, t.name, t.definition, opts), "unable to build the container: %s", t.name); err != nil {
				return
			}
		}
	})

	return err
}

func (t ContainerRunner) RunWith(ctx context.Context, mpath string) (err error) {
	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	return errorsx.Wrapf(ffiegcontainer.Run(ctx, t.name, mpath, t.cmd, opts), "unable to run the container: %s", t.name)
}

// internal use.
func (t ContainerRunner) ToModuleRunner() ContainerModuleRunner {
	return ContainerModuleRunner{ContainerRunner: t}
}

func (t ContainerRunner) OptionWorkingDirectory(dir string) ContainerRunner {
	t.options = append(t.options, (coption{}).workdir(dir))
	return t
}

func (t ContainerRunner) OptionEnvVar(k string) ContainerRunner {
	t.options = append(t.options, (coption{}).envvar(k, ""))
	return t
}

func (t ContainerRunner) OptionEnv(k, v string) ContainerRunner {
	t.options = append(t.options, (coption{}).envvar(k, v))
	return t
}

type ContainerModuleRunner struct {
	ContainerRunner
}

func (t ContainerModuleRunner) RunWith(ctx context.Context, mpath string) (err error) {
	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	opts = append(opts, "-e", fmt.Sprintf("%s=%d", eg.EnvComputeModuleNestedLevel, envx.Int(-1, eg.EnvComputeModuleNestedLevel)+1))

	return errorsx.Wrapf(ffiegcontainer.Module(ctx, t.name, mpath, opts), "unable to run the module: %s", t.name)
}

func Build(r Runner) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.CompileWith(ctx)
	}
}

// Module executes a set of operations within the provided environment.
// Important: this method acts as an Instrumentation point by the runtime.
func Module(ctx context.Context, r Runner, references ...OpFn) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.CompileWith(ctx)
	}
}

// Exec executes command with the given runner
// Important: this method acts as an Instrumentation point by the runtime.
func Exec(ctx context.Context, r Runner) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.CompileWith(ctx)
	}
}

// Deprecated: this is intended for internal use only. do not use.
// used to replace invocations at runtime.
func UnsafeRunner(ctx context.Context, r Runner, modulepath string) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.RunWith(ctx, modulepath)
	}
}

// Deprecated: this is intended for internal use only. do not use.
// used to replace the exec invocations at runtime.
func UnsafeExec(ctx context.Context, r Runner, modulepath string) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.RunWith(ctx, modulepath)
	}
}
