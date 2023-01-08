package yak

import (
	"context"
	"errors"
	"log"

	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffiexec"
)

// represents a sequence of operations to perform.
type Task interface {
	Do(ctx context.Context) error
}

type Op interface {
	// Context() context.Context TODO
}
type op func(Op) error

type fnTask func(ctx context.Context) error

func (t fnTask) Do(ctx context.Context) error {
	return t(ctx)
}

func Module(o func(ctx context.Context) error) Task {
	return fnTask(func(ctx context.Context) error {
		return o(ctx)
	})
}

func Perform(ctx context.Context, tasks ...Task) error {
	for _, t := range tasks {
		if err := t.Do(ctx); err != nil {
			return err
		}
	}

	return nil
}

func Deferred(tasks ...Task) Task {
	return fnTask(func(ctx context.Context) error {
		for _, task := range tasks {
			if err := task.Do(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func Sequential(operations ...op) Task {
	return fnTask(func(ctx context.Context) error {
		for _, op := range operations {
			if err := op(nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func Parallel(operations ...op) Task {
	return fnTask(func(ctx context.Context) error {
		for _, op := range operations {
			if err := op(nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func When(b bool, o Task) Task {
	return fnTask(func(ctx context.Context) error {
		if !b {
			return nil
		}

		return o.Do(ctx)
	})
}

type Runner interface {
	Perform(...Task) error
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

func (t ContainerRunner) DefinitionFile(s string) ContainerRunner {
	t.definition = s
	return t
}

func (t ContainerRunner) Perform(ctx context.Context, tasks ...Task) (err error) {
	var (
		setup []Task
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

	return Perform(ctx, Deferred(setup...), Deferred(tasks...))
}
