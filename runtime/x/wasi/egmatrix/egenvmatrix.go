package egmatrix

import (
	"iter"
	"net/netip"
	"time"
)

// generates every permutation of provided options by assigning the values to their respective fields on T.
// example:
//
//	type Foo struct{
//		Field1 string
//		Field2 bool
//	}
//
// egmatrix.New(Foo{}).
//
//	Boolean(func(d *Foo, v bool) {d.Field2 = v}).
//	String(func(d*Foo, v string) {d.Field1 = v},)
type Builder[T any] interface {
	Boolean(func(dst *T, v bool)) Builder[T]
	String(m func(dst *T, v string), options ...string) Builder[T]
	Int(m func(dst *T, v int), options ...int) Builder[T]
	Int8(m func(dst *T, v int8), options ...int8) Builder[T]
	Int16(m func(dst *T, v int16), options ...int16) Builder[T]
	Int32(m func(dst *T, v int32), options ...int32) Builder[T]
	Int64(m func(dst *T, v int64), options ...int64) Builder[T]
	Uint(m func(dst *T, v uint), options ...uint) Builder[T]
	Uint8(m func(dst *T, v uint8), options ...uint8) Builder[T]
	Uint16(m func(dst *T, v uint16), options ...uint16) Builder[T]
	Uint32(m func(dst *T, v uint32), options ...uint32) Builder[T]
	Uint64(m func(dst *T, v uint64), options ...uint64) Builder[T]
	Float32(m func(dst *T, v float32), options ...float32) Builder[T]
	Float64(m func(dst *T, v float64), options ...float64) Builder[T]
	Duration(m func(dst *T, v time.Duration), options ...time.Duration) Builder[T]
	Time(m func(dst *T, v time.Time), options ...time.Time) Builder[T]
	Addr(m func(dst *T, v netip.Addr), options ...netip.Addr) Builder[T]
	Prefix(m func(dst *T, v netip.Prefix), options ...netip.Prefix) Builder[T]
	Perm() iter.Seq[T]
}

type M[T any] struct {
	mutations [][]func(*T)
}

func New[T any]() *M[T] {
	return &M[T]{}
}

func Assign[T any, V any](m *M[T], fn func(dst *T, v V), options ...V) *M[T] {
	group := make([]func(*T), 0, len(options))
	for _, opt := range options {
		group = append(group, func(dst *T) { fn(dst, opt) })
	}
	m.mutations = append(m.mutations, group)
	return m
}

func (m *M[T]) Boolean(fn func(dst *T, v bool)) Builder[T] {
	return Assign(m, fn, true, false)
}

func (m *M[T]) String(fn func(dst *T, v string), options ...string) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Int64(fn func(dst *T, v int64), options ...int64) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Int(fn func(dst *T, v int), options ...int) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Int8(fn func(dst *T, v int8), options ...int8) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Int16(fn func(dst *T, v int16), options ...int16) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Int32(fn func(dst *T, v int32), options ...int32) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Uint(fn func(dst *T, v uint), options ...uint) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Uint8(fn func(dst *T, v uint8), options ...uint8) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Uint16(fn func(dst *T, v uint16), options ...uint16) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Uint32(fn func(dst *T, v uint32), options ...uint32) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Uint64(fn func(dst *T, v uint64), options ...uint64) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Float32(fn func(dst *T, v float32), options ...float32) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Float64(fn func(dst *T, v float64), options ...float64) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Duration(fn func(dst *T, v time.Duration), options ...time.Duration) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Time(fn func(dst *T, v time.Time), options ...time.Time) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Addr(fn func(dst *T, v netip.Addr), options ...netip.Addr) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Prefix(fn func(dst *T, v netip.Prefix), options ...netip.Prefix) Builder[T] {
	return Assign(m, fn, options...)
}

func (m *M[T]) Perm() iter.Seq[T] {
	return func(yield func(T) bool) {
		if len(m.mutations) == 0 {
			return
		}

		// Generate cartesian product of all mutation groups
		indices := make([]int, len(m.mutations))
		m.generate(indices, 0, yield)
	}
}

func (m *M[T]) generate(indices []int, depth int, yield func(T) bool) bool {
	if depth == len(m.mutations) {
		var result T
		for i, idx := range indices {
			m.mutations[i][idx](&result)
		}
		return yield(result)
	}

	for i := 0; i < len(m.mutations[depth]); i++ {
		indices[depth] = i
		if !m.generate(indices, depth+1, yield) {
			return false
		}
	}
	return true
}
