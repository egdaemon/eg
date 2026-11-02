// Package cmpx builds comparison functions by converting both values to a byte vector, one key at a
// time, and comparing the vectors. the first key that differs decides the order.
package cmpx

import (
	"bytes"
	"slices"
)

// C compares a value against another using the keys added to it.
type C[T any] struct {
	v    T
	keys []func(v T) []byte
}

// For starts a comparison of values of type T, add keys then use Compare, which is usable directly as a sort function.
func For[T any]() C[T] {
	return C[T]{}
}

// From starts a comparison with v as the left hand side, see With.
func From[T any](v T) C[T] {
	return C[T]{v: v}
}

// Bool adds a key that orders values for which f is true before those for which it is false.
func (t C[T]) Bool(f func(v T) bool) C[T] {
	return t.key(func(v T) []byte {
		if f(v) {
			return []byte{0}
		}

		return []byte{1}
	})
}

// String adds a key that orders values by f, ascending.
func (t C[T]) String(f func(v T) string) C[T] {
	return t.key(func(v T) []byte {
		// terminated so a shorter string orders before its extensions without bleeding into the next key,
		// zero bytes are escaped so the terminator is unambiguous.
		s := f(v)
		out := make([]byte, 0, len(s)+2)
		for i := 0; i < len(s); i++ {
			if s[i] == 0 {
				out = append(out, 0, 0xFF)
				continue
			}
			out = append(out, s[i])
		}

		return append(out, 0, 1)
	})
}

// With compares the value the comparison started from with b, returning -1, 0, or +1. suitable for slices.SortFunc.
func (t C[T]) With(b T) int {
	return t.Compare(t.v, b)
}

// Compare a and b using the keys, returning -1, 0, or +1. the method value is a func(a, b T) int, suitable for slices.SortFunc.
func (t C[T]) Compare(a, b T) int {
	var x, y []byte
	for _, k := range t.keys {
		x, y = append(x, k(a)...), append(y, k(b)...)
	}

	return bytes.Compare(x, y)
}

func (t C[T]) key(f func(v T) []byte) C[T] {
	t.keys = append(slices.Clone(t.keys), f)
	return t
}
