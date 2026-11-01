// Package egerrors provides combinators for handling errors returned by operations.
package egerrors

import (
	"context"
	"errors"

	"github.com/egdaemon/eg/runtime/wasi/eg"
)

func matches(err error, errs []error) bool {
	if len(errs) == 0 {
		return true
	}

	for _, target := range errs {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}

// Ignore errors returned by the operation that match any of the provided errors (errors.Is).
// if no errors are provided then all errors are ignored.
func Ignore(op eg.Runnable, errs ...error) eg.OpFn {
	return When(op, eg.Fn(func(context.Context, eg.Op) error { return nil }), errs...)
}

// When the operation fails with an error matching any of the provided errors (errors.Is)
// run the 'do' operation, the result of 'do' replaces the original error.
// if no errors are provided then any error triggers 'do'. errors that don't match are returned as is.
func When(op eg.Runnable, do eg.Runnable, errs ...error) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		err := op.Op(ctx, o)
		if err == nil || !matches(err, errs) {
			return err
		}

		return do.Op(ctx, o)
	}
}

// Chain runs the operation, if it fails each fallback is attempted in order
// until one succeeds. if all of them fail then the errors are joined and returned.
func Chain(op eg.Runnable, fallbacks ...eg.Runnable) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		err := op.Op(ctx, o)
		if err == nil {
			return nil
		}

		failures := []error{err}
		for _, fallback := range fallbacks {
			if err = ctx.Err(); err != nil {
				return errors.Join(append(failures, err)...)
			}

			if err = fallback.Op(ctx, o); err == nil {
				return nil
			}

			failures = append(failures, err)
		}

		return errors.Join(failures...)
	}
}
