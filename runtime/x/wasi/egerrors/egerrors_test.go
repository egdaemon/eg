package egerrors_test

import (
	"context"
	"errors"
	"testing"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egerrors"
)

var (
	errA = errors.New("a")
	errB = errors.New("b")
)

func fail(err error) eg.OpFn {
	return func(context.Context, eg.Op) error { return err }
}

func run(t *testing.T, op eg.OpFn) error {
	t.Helper()
	return op(context.Background(), nil)
}

func TestIgnore(t *testing.T) {
	if err := run(t, egerrors.Ignore(fail(errA), errA)); err != nil {
		t.Fatal(err)
	}
	if err := run(t, egerrors.Ignore(fail(errA))); err != nil {
		t.Fatal(err)
	}
	if err := run(t, egerrors.Ignore(fail(errB), errA)); !errors.Is(err, errB) {
		t.Fatalf("expected errB got %v", err)
	}
}

func TestWhen(t *testing.T) {
	called := false
	do := eg.Fn(func(context.Context, eg.Op) error { called = true; return nil })

	if err := run(t, egerrors.When(fail(errB), do, errA)); !errors.Is(err, errB) || called {
		t.Fatalf("unmatched error must pass through: %v %v", err, called)
	}
	if err := run(t, egerrors.When(fail(errA), do, errA)); err != nil || !called {
		t.Fatalf("matched error must run do: %v %v", err, called)
	}
	called = false
	if err := run(t, egerrors.When(fail(nil), do)); err != nil || called {
		t.Fatalf("success must not run do: %v %v", err, called)
	}
}

func TestChain(t *testing.T) {
	if err := run(t, egerrors.Chain(fail(errA), fail(errB), fail(nil))); err != nil {
		t.Fatal(err)
	}
	err := run(t, egerrors.Chain(fail(errA), fail(errB)))
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("expected joined errors got %v", err)
	}
}
