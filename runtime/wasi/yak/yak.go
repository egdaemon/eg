package yak

import (
	"context"
	"errors"
	"fmt"
	"log"
	"unsafe"

	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffiexec"
)

// represents a sequence of operations to perform.
type task interface {
	Do(ctx context.Context) error
}

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
// invocations of this method are often replaced during build time with a custom
// implementation by the runtime
func Ref(o op) Reference {
	addr := *(*uintptr)(unsafe.Pointer(&o))
	return runtimeref{ptr: addr, do: o}
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
	return fnTask(func(ctx context.Context) error {
		for _, op := range operations {
			if err := op(ctx, Ref(op)); err != nil {
				return err
			}
		}
		return nil
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
	Module(ctx context.Context, references ...Reference) (err error)
}

// Run the tasks with the specified container.
func Container(name string) ContainerRunner {
	return ContainerRunner{
		tmpdir: envx.String("", "EG_RUNTIME_DIRECTORY"),
		name:   name,
	}
}

type ContainerRunner struct {
	name       string
	tmpdir     string
	definition string
}

func (t ContainerRunner) BuildFromFile(s string) ContainerRunner {
	t.definition = s
	return t
}

func (t ContainerRunner) Module(ctx context.Context, references ...Reference) (err error) {
	var (
		setup []task
	)

	// lookup container from registry
	// if not found fallback to the definition
	// if no definition then we have an error
	if t.definition != "" {
		// sudo podman build -t localhost/derp:latest -f ./zderp/custom/Containerfile -f ./zderp/egci/Containerfile ./zderp/egci/
		// sudo podman run --detach --name derpy --volume ./egci/.filesystem:/opt localhost/derp:latest /usr/sbin/init
		log.Printf("building container %s\n", t.definition)
		if code := ffiexec.Command("podman", []string{"build", "-f", t.definition, ".egci"}); code != 0 {
			return errors.New("unable to build the container")
		}
	}

	for _, r := range references {
		log.Println("reference", r.ID())
	}

	return Perform(ctx, deferred(setup...))
}

func deferred(tasks ...task) task {
	return fnTask(func(ctx context.Context) error {
		for _, task := range tasks {
			if err := task.Do(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}
