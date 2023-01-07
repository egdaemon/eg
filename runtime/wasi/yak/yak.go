package yak

import (
	"bytes"
	"context"
	"io"
	"log"
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
		name: name,
	}
}

type ContainerRunner struct {
	name       string
	definition io.Reader
}

func (t ContainerRunner) Definition(src io.Reader) ContainerRunner {
	t.definition = src
	return t
}

func (t ContainerRunner) Perform(tasks ...Task) error {
	var (
		b bytes.Buffer
	)

	// lookup container from registry
	// if not found fallback to the definition
	// if no definition then we have an error
	if t.definition != nil {
		if _, err := io.Copy(&b, t.definition); err != nil {
			return err
		}

		log.Printf("building container %s\n%s\n", t.name, b.String())
	}

	return Perform(context.Background(), tasks...)
}
