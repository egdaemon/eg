package yak

import (
	"bytes"
	"io"
	"log"
)

// represents a sequence of operations to perform.
type Task interface {
	Do() error
}

type Op interface{}
type op func(Op) error

func Module(o func() error) Task {
	return fnTask(func() error {
		return o()
	})
}

func Perform(tasks ...Task) error {
	for _, t := range tasks {
		if err := t.Do(); err != nil {
			return err
		}
	}

	return nil
}

func Deferred(tasks ...Task) Task {
	return fnTask(func() error {
		for _, task := range tasks {
			if err := task.Do(); err != nil {
				return err
			}
		}
		return nil
	})
}

type fnTask func() error

func (t fnTask) Do() error {
	return t()
}

func Sequential(operations ...op) Task {
	return fnTask(func() error {
		for _, op := range operations {
			if err := op(nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func Parallel(operations ...op) Task {
	return fnTask(func() error {
		for _, op := range operations {
			if err := op(nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func When(b bool, o Task) Task {
	return fnTask(func() error {
		if !b {
			return nil
		}

		return o.Do()
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

	return Perform(tasks...)
}
