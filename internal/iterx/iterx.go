package iterx

import (
	"context"
	"iter"
)

// Seq is an iterator over values of type T that also carries an error.
type Seq[T any] interface {
	Each(context.Context) iter.Seq[T]
	Err() error
}

type seq[T any] struct {
	err error
	fn  func(ctx context.Context, yield func(T) bool) error
}

// New constructs a Seq[T] from a function that drives iteration and returns any
// error that occurred. The error is available via Err() after Each is consumed.
func New[T any](fn func(ctx context.Context, yield func(T) bool) error) Seq[T] {
	return &seq[T]{fn: fn}
}

func (s *seq[T]) Each(ctx context.Context) iter.Seq[T] {
	return func(yield func(T) bool) {
		s.err = s.fn(ctx, yield)
	}
}

func (s *seq[T]) Err() error {
	return s.err
}

// Error returns a Seq that yields nothing and immediately returns err.
func Error[T any](err error) Seq[T] {
	return New[T](func(ctx context.Context, yield func(T) bool) error {
		return err
	})
}
