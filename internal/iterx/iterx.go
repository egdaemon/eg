package iterx

import (
	"context"
	"iter"
	"time"
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

// N returns a Seq that yields at most n items of s, 0 yields all of them. s is stopped as soon as the nth item has
// been yielded, without reading another, and its error is the error of the result.
func N[T any](n uint, s Seq[T]) Seq[T] {
	return New(func(ctx context.Context, yield func(T) bool) error {
		var count uint
		for item := range s.Each(ctx) {
			if !yield(item) {
				break
			}

			count++
			if n > 0 && count >= n {
				break
			}
		}

		return s.Err()
	})
}

// Until returns a Seq that stops yielding the items of s once less than grace remains before the deadline of the
// context it is consumed with, leaving the item in flight time to finish. a context without a deadline yields every
// item. the error of s is the error of the result.
func Until[T any](grace time.Duration, s Seq[T]) Seq[T] {
	return New(func(ctx context.Context, yield func(T) bool) error {
		deadline, ok := ctx.Deadline()

		for item := range s.Each(ctx) {
			if ok && time.Until(deadline) < grace {
				break
			}

			if !yield(item) {
				break
			}
		}

		return s.Err()
	})
}

// Error returns a Seq that yields nothing and immediately returns err.
func Error[T any](err error) Seq[T] {
	return New(func(ctx context.Context, yield func(T) bool) error {
		return err
	})
}

// Slice returns a Seq that yields the given items in order.
func Slice[T any](items ...T) Seq[T] {
	return New(func(ctx context.Context, yield func(T) bool) error {
		for _, item := range items {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			if !yield(item) {
				return nil
			}
		}

		return nil
	})
}
