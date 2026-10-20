package sqlx

import (
	"database/sql"
	"iter"

	"github.com/egdaemon/eg/internal/errorsx"
)

type Scanner[T any] interface {
	Scan(i *T) error
	Next() bool
	Close() error
	Err() error
}

type rowsscanner[T any] struct {
	rows *sql.Rows
}

func (r *rowsscanner[T]) Scan(i *T) error { return r.rows.Scan(i) }
func (r *rowsscanner[T]) Next() bool      { return r.rows.Next() }
func (r *rowsscanner[T]) Close() error    { return r.rows.Close() }
func (r *rowsscanner[T]) Err() error      { return r.rows.Err() }

func NewRowsScanner[T any](rows *sql.Rows, err error) Scanner[T] {
	if err != nil {
		return errorScanner[T]{cause: err}
	}
	return &rowsscanner[T]{rows: rows}
}

type errorScanner[T any] struct{ cause error }

func (e errorScanner[T]) Scan(_ *T) error { return e.cause }
func (e errorScanner[T]) Next() bool      { return false }
func (e errorScanner[T]) Close() error    { return nil }
func (e errorScanner[T]) Err() error      { return e.cause }

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
			var (
				p T
			)

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

func Scan[T any](s Scanner[T]) Iter[T] {
	return &scanningiter[T]{
		s: s,
	}
}

func ScanOne[T any](s Scanner[T]) (_zero T, _ error) {
	i := Scan(s)

	for v := range i.Iter() {
		return v, i.Err()
	}

	return _zero, errorsx.Compact(i.Err(), sql.ErrNoRows)
}

// ScanInto a slice, automatically closes the scanner once done.
func ScanInto[T any](s Scanner[T], dst *[]T) (err error) {
	i := Scan(s)
	for v := range i.Iter() {
		*dst = append(*dst, v)
	}

	return i.Err()
}

func Discard[T any](s Iter[T]) (err error) {
	for range s.Iter() {
	}

	return s.Err()
}

func Discarded[T any](s Iter[T]) (c uint64, err error) {
	for range s.Iter() {
		c += 1
	}

	return c, s.Err()
}
