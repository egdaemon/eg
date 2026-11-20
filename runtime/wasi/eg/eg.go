package eg

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigraph"
)

// Generally unsafe predefined runner for modules. useful
// for providing a base line environment but has no long term
// stability promises.
func DefaultModule() C8S {
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

// Op implements Runnable so every OpFn is a Runnable.
func (t OpFn) Op(ctx context.Context, o Op) error {
	return t(ctx, o)
}

// Runnable is the interface form of an OpFn. it allows richer types to be embedded
// in other interfaces. pass the method value (r.Op) where an OpFn is expected.
type Runnable interface {
	Op(context.Context, Op) error
}

// Fn adapts a bare function into an OpFn, and therefore a Runnable.
func Fn(fn func(context.Context, Op) error) OpFn {
	return OpFn(fn)
}

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

// Run operations in sequence
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

		for range operations {
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

// C is the compilation half of the virtualization layer, it prepares the runtime (build or pull a container, etc).
type C interface {
	CompileWith(ctx context.Context) (err error)
}

// V is the virtualization layer, used to represent containers, vms, or other such runtimes.
type V interface {
	RunWith(ctx context.Context, mpath string) (err error)
}

// v is both halves of the virtualization layer.
type v interface {
	V
	C
}

// Deprecated: use V and C.
type Runner = v

func Build(r C) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.CompileWith(ctx)
	}
}

// Module executes a set of operations within the provided environment.
// Important: this method acts as an Instrumentation point by the runtime.
func Module(ctx context.Context, r v, references ...OpFn) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.CompileWith(ctx)
	}
}

// Exec executes command with the given runner
// Important: this method acts as an Instrumentation point by the runtime.
func Exec(ctx context.Context, r v) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.CompileWith(ctx)
	}
}

// Deprecated: this is intended for internal use only. do not use.
// used to replace invocations at runtime.
func UnsafeRunner(ctx context.Context, r V, modulepath string) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.RunWith(ctx, modulepath)
	}
}

// Deprecated: this is intended for internal use only. do not use.
// used to replace the exec invocations at runtime.
func UnsafeExec(ctx context.Context, r V, modulepath string) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.RunWith(ctx, modulepath)
	}
}
