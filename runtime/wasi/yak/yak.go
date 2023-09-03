package yak

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"unsafe"

	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/internal/stringsx"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiguest"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffiegcontainer"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffigraph"
)

// A reference to an operation.
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
	addr := *(*uintptr)(unsafe.Pointer(&o))
	return runtimeref{ptr: addr, do: o}
}

type transpiledref struct {
	name string
	do   OpFn
}

func (t transpiledref) ID() string {
	return t.name
}

// Deprecated: this is intended for internal use only. do not use.
// its use may prevent future builds from executing.
func UnsafeTranspiledRef(name string, o OpFn) Reference {
	return transpiledref{
		name: name,
		do:   o,
	}
}

func Perform(ctx context.Context, tasks ...OpFn) error {
	return ffigraph.WrapErr(namedop("perform"), func() error {
		for _, t := range tasks {
			if err := t(ctx, ref(t)); err != nil {
				return err
			}
		}

		return nil
	})
}

func Sequential(operations ...OpFn) OpFn {
	return func(ctx context.Context, o Op) error {
		return ffigraph.WrapErr(prefixedop("seq", o), func() error {
			for _, op := range operations {
				r := ref(op)
				err := ffigraph.WrapErr(r, func() error {
					return op(ctx, r)
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
}

// Run operations in parallel.
// WARNING: currently due to limitations within wasi runtimes
// threading isn't supported. this makes parallelism impossible
// natively within the runtime; however some operations like executing
// modules can be done in parallel since they are manage on the host
// and not inside the runtime. in the future when wasi environments
// gain threading this will automatically begin running operations
// in parallel natively. to prevent issues in the future we shuffle
// operations to ensure callers are not implicitly relying on order.
func Parallel(operations ...OpFn) OpFn {
	return func(ctx context.Context, o Op) (err error) {
		return ffigraph.WrapErr(prefixedop("par", o), func() error {
			errs := make(chan error, len(operations))
			defer close(errs)

			rand.Shuffle(len(operations), func(i, j int) {
				operations[i], operations[j] = operations[j], operations[i]
			})

			for _, o := range operations {
				go func(iop OpFn) {
					r := ref(iop)
					select {
					case <-ctx.Done():
						errs <- ctx.Err()
					case errs <- ffigraph.WrapErr(r, func() error { return iop(ctx, r) }):
					}
				}(o)
			}

			for i := 0; i < len(operations); i++ {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case cause := <-errs:
					err = errorsx.Compact(err, cause)
				}
			}

			return err
		})
	}
}

func When(b bool, o OpFn) OpFn {
	return func(ctx context.Context, i Op) error {
		if !b {
			return nil
		}

		return o(ctx, ref(o))
	}
}

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

func (t coption) volume(host, guest, opts string) coption {
	return []string{"--volume", fmt.Sprintf("%s:%s:%s", host, guest, opts)}
}

func (t coption) privileged() coption {
	return []string{"--privileged"}
}

func (t coption) user(user string) coption {
	return []string{"--user", user}
}

type ContainerRunner struct {
	name       string
	definition string
	pull       string
	options    []coption
	built      *sync.Once
}

func (t ContainerRunner) BuildFromFile(s string) ContainerRunner {
	t.definition = s
	return t
}

func (t ContainerRunner) PullFrom(s string) ContainerRunner {
	t.pull = s
	return t
}

// CompileWith builds the container and
func (t ContainerRunner) CompileWith(ctx context.Context) (err error) {
	if ffigraph.Analysing() {
		return nil
	}

	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	t.built.Do(func() {
		if t.pull != "" {
			err = ffiguest.Error(ffiegcontainer.Pull(t.pull, opts), fmt.Errorf("unable to pull the container: %s", t.name))
		}

		if t.definition != "" {
			err = ffiguest.Error(ffiegcontainer.Build(t.name, t.definition, opts), fmt.Errorf("unable to build the container: %s", t.name))
		}
	})

	return err
}

func (t ContainerRunner) RunWith(ctx context.Context, mpath string) (err error) {
	var opts []string
	for _, o := range t.options {
		opts = append(opts, o...)
	}

	return ffiguest.Error(ffiegcontainer.Module(t.name, mpath, opts), fmt.Errorf("unable to run the container: %s", t.name))
}

func (t ContainerRunner) OptionPrivileged() ContainerRunner {
	t.options = append(t.options, (coption{}).privileged())
	return t
}

func (t ContainerRunner) OptionUser(username string) ContainerRunner {
	t.options = append(t.options, (coption{}).user(username))
	return t
}

// Mount a directory into the container at the provided host, guest paths as read only.
func (t ContainerRunner) OptionVolume(host, guest string) ContainerRunner {
	t.options = append(t.options, (coption{}).volume(host, guest, "ro"))
	return t
}

// Mount a directory into the container at the provided the host, guest paths and the mount permissions.
// the mount permissions default to read only when an empty string
func (t ContainerRunner) OptionVolumeWithPermissions(host, guest, perms string) ContainerRunner {
	t.options = append(t.options, (coption{}).volume(host, guest, stringsx.DefaultIfBlank(perms, "ro")))
	return t
}

// Module executes a set of operations within the provided environment.
// Important: this method acts as an Instrumentation point by the runtime.
func Module(ctx context.Context, r Runner, references ...OpFn) OpFn {
	return func(ctx context.Context, o Op) error {
		return r.CompileWith(ctx)
	}
}

// Deprecated: this is intended for internal use only. do not use.
// used to replace the module invocations at runtime.
func UnsafeModule(ctx context.Context, r Runner, modulepath string) OpFn {
	if err := r.CompileWith(ctx); err != nil {
		return func(context.Context, Op) error {
			return err
		}
	}

	return func(ctx context.Context, o Op) error {
		return r.RunWith(ctx, modulepath)
	}
}
