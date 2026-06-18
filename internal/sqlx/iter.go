package sqlx

import (
	"iter"

	"github.com/egdaemon/eg/internal/errorsx"
)

// Scanner is the minimal interface genieql-generated *Scanner types satisfy.
type Scanner[T any] interface {
	Scan(i *T) error
	Next() bool
	Close() error
	Err() error
}

// Iter adapts a Scanner into a range-able iter.Seq, closing the scanner once
// iteration completes (whether exhausted, errored, or stopped early).
type Iter[T any] interface {
	Iter() iter.Seq[T]
	Err() error
}

type scanningiter[T any] struct {
	s     Scanner[T]
	cause error
}

func (t *scanningiter[T]) Iter() iter.Seq[T] {
	return func(yield func(T) bool) {
		defer func() {
			t.cause = errorsx.Compact(t.cause, t.s.Close())
		}()

		for t.s.Next() {
			var p T

			if err := t.s.Scan(&p); err != nil {
				t.cause = errorsx.WithStack(err)
				return
			}

			if !yield(p) {
				return
			}
		}

		t.cause = t.s.Err()
	}
}

func (t *scanningiter[T]) Err() error {
	return t.cause
}

// Scan wraps a Scanner into an Iter.
func Scan[T any](s Scanner[T]) Iter[T] {
	return &scanningiter[T]{
		s: s,
	}
}

// ScanInto a slice, automatically closes the scanner once done.
func ScanInto[T any](s Scanner[T], dst *[]T) (err error) {
	i := Scan(s)
	for v := range i.Iter() {
		*dst = append(*dst, v)
	}

	return i.Err()
}

// Discard drains the scanner without retaining results.
func Discard[T any](s Scanner[T]) (err error) {
	i := Scan(s)
	for range i.Iter() {
	}

	return i.Err()
}
