package yak

import (
	"context"
	"fmt"
	"log"
	"unsafe"

	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/md5x"
	"github.com/james-lawrence/eg/runtime/wasi/internal/ffiexec"
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
	CompileWith(ctx context.Context) (err error)
	RunWith(ctx context.Context, mpath string) (err error)
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

// CompileWith builds the container and
func (t ContainerRunner) CompileWith(ctx context.Context) (err error) {
	log.Printf("building container %s\n", t.definition)
	if code := ffiexec.Command("podman", []string{"build", "--timestamp", "0", "-t", t.name, "-f", t.definition, ".egci"}); code != 0 {
		return errors.New("unable to build the container")
	}

	return nil
}

func (t ContainerRunner) RunWith(ctx context.Context, mpath string) (err error) {
	// TODO: implement a custom host method. this is currently a security risk
	if code := ffiexec.Command("podman", []string{
		"run",
		"--name",
		fmt.Sprintf("%s.%s", t.name, md5x.DigestString(mpath)),
		"--detach",
		"--volume",
		fmt.Sprintf("%s:/opt/egmodule.wasm:O", mpath),
		"--volume",
		fmt.Sprintf("%s:/opt/eg:O", envx.String("", "EG_ROOT_DIRECTORY")),
		"--volume",
		fmt.Sprintf("%s:/opt/egbin:ro", "/home/james.lawrence/go/bin/eg"),
		t.name,
		"/usr/sbin/init",
	}); code != 0 {
		return errors.New("unable to build the container")
	}

	return nil
}

// Module executes a set of references within the provided environment.
// Important: this method acts as an Instrumentation point by the runtime.
func Module(ctx context.Context, r Runner, references ...op) error {
	// generate a module main file based on the references.
	log.Println("generating a module with", len(references), "references")
	return r.CompileWith(ctx)
}

// Deprecated: this is intended for internal use only. do not use.
// used to replace the module invocations at runtime.
func UnsafeModule(ctx context.Context, r Runner, modulepath string) error {
	return r.RunWith(ctx, modulepath)
}
