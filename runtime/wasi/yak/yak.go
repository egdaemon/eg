package yak

import (
	"context"
	"fmt"
	"log"
	"sync"
	"unsafe"

	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffiegcontainer"
	"github.com/pkg/errors"
)

// represents a sequence of operations to perform.
type task interface {
	Do(ctx context.Context) error
}

// A reference to an operation.
type Reference interface {
	ID() string
}

type Op interface {
	ID() string
}
type op func(context.Context, Op) error

type fnTask func(ctx context.Context) error

func (t fnTask) Do(ctx context.Context) error {
	return t(ctx)
}

type runtimeref struct {
	ptr uintptr
	do  op
}

func (t runtimeref) ID() string {
	return fmt.Sprintf("%x", t.ptr)
}

// Ref meta programming marking a task for delayed execution when rewriting the program at compilation time.
// if executed directly will use the memory location of the function.
// Important: this method acts as an instrumentation point by the runtime.
func Ref(o op) Reference {
	addr := *(*uintptr)(unsafe.Pointer(&o))
	return runtimeref{ptr: addr, do: o}
}

type transpiledref struct {
	name string
	do   op
}

func (t transpiledref) ID() string {
	return t.name
}

// Deprecated: this is intended for internal use only. do not use.
// its use may prevent future builds from executing.
func UnsafeTranspiledRef(name string, o op) Reference {
	return transpiledref{
		name: name,
		do:   o,
	}
}

func Perform(ctx context.Context, tasks ...task) error {
	for _, t := range tasks {
		if err := t.Do(ctx); err != nil {
			return err
		}
	}

	return nil
}

func Sequential(operations ...op) task {
	return fnTask(func(ctx context.Context) error {
		for _, op := range operations {
			if err := op(ctx, Ref(op)); err != nil {
				return err
			}
		}
		return nil
	})
}

func Parallel(operations ...op) task {
	return fnTask(func(ctx context.Context) (err error) {
		errs := make(chan error, len(operations))
		defer close(errs)

		for _, o := range operations {
			go func(iop op) {
				select {
				case <-ctx.Done():
					errs <- ctx.Err()
				case errs <- iop(ctx, Ref(iop)):
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

func When(b bool, o task) task {
	return fnTask(func(ctx context.Context) error {
		if !b {
			return nil
		}

		return o.Do(ctx)
	})
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

type ContainerRunner struct {
	name       string
	definition string
	built      *sync.Once
}

func (t ContainerRunner) BuildFromFile(s string) ContainerRunner {
	t.definition = s
	return t
}

// CompileWith builds the container and
func (t ContainerRunner) CompileWith(ctx context.Context) (err error) {
	t.built.Do(func() {
		log.Printf("building container %s\n", t.definition)
		if code := ffiegcontainer.Build(t.name, t.definition); code != 0 {
			err = errors.Errorf("unable to build the container: %d", code)
			return
		}
	})

	return err
}

func (t ContainerRunner) RunWith(ctx context.Context, mpath string) (err error) {
	if code := ffiegcontainer.Run(t.name, mpath); code != 0 {
		return errors.Errorf("unable to build the container: %d", code)
	}

	return nil
}

// Module executes a set of operations within the provided environment.
// Important: this method acts as an Instrumentation point by the runtime.
func Module(ctx context.Context, r Runner, references ...op) op {
	return func(ctx context.Context, o Op) error {
		return r.CompileWith(ctx)
	}
}

// Deprecated: this is intended for internal use only. do not use.
// used to replace the module invocations at runtime.
func UnsafeModule(ctx context.Context, r Runner, modulepath string) op {
	if err := r.CompileWith(ctx); err != nil {
		return func(context.Context, Op) error {
			return err
		}
	}

	return func(ctx context.Context, o Op) error {
		return r.RunWith(ctx, modulepath)
	}
}
