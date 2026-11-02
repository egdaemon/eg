// Package cmpx builds comparison functions by converting both values to a byte vector, one key at a
// time, and comparing the vectors. the first key that differs decides the order.
//
// keys: Cmp and Rev (descending) for any Ordered type (including named types such as time.Duration), Bytes and
// Time. S adds By for keys that are fields of a struct.
//
//	cmpx.New[Foo]().Cmp(func(v Foo) int { return v.Weight }).Cmp(func(v Foo) string { return v.Name })
//
//	var z Foo
//	cmpx.For(&z).By(&z.Weight).By(&z.Name)   // compares Foo
//	cmpx.From(&z).By(&z.Weight).By(&z.Name)  // compares *Foo
package cmpx

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"math"
	"reflect"
	"slices"
	"time"
)

// Ordered the types that can be used as a Cmp, Rev or By key. bool orders false before true.
type Ordered interface {
	cmp.Ordered | ~bool
}

// C compares values using the keys added to it. the zero value is ready to use, an empty comparison treats every value as equal.
type C[T any] struct {
	v    T
	keys []func(v T) []byte
}

// New starts a comparison of values of type T, add keys then use Compare, which is usable directly as a sort function.
func New[T any]() C[T] {
	return C[T]{}
}

// Bytes adds a key that orders values by f, ascending.
func (t C[T]) Bytes(f func(v T) []byte) C[T] {
	return t.key(func(v T) []byte {
		return terminated(f(v))
	})
}

// Time adds a key that orders values by the instant f returns, ascending. the location is ignored.
func (t C[T]) Time(f func(v T) time.Time) C[T] {
	return t.key(func(v T) []byte {
		ts := f(v)
		return binary.BigEndian.AppendUint32(int64Key(ts.Unix()), uint32(ts.Nanosecond()))
	})
}

// Cmp adds a key that orders values by f, ascending. the key may be any ordered type, including
// named types such as time.Duration. floats order like cmp.Compare: NaN before everything, -0 equal to +0.
func (t C[T]) Cmp[R Ordered](f func(v T) R) C[T] {
	switch reflect.TypeFor[R]().Kind() {
	case reflect.Bool:
		return t.key(func(v T) []byte {
			if reflect.ValueOf(f(v)).Bool() {
				return []byte{1}
			}

			return []byte{0}
		})
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return t.key(func(v T) []byte { return int64Key(reflect.ValueOf(f(v)).Int()) })
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return t.key(func(v T) []byte { return binary.BigEndian.AppendUint64(nil, reflect.ValueOf(f(v)).Uint()) })
	case reflect.Float32, reflect.Float64:
		return t.key(func(v T) []byte { return float64Key(reflect.ValueOf(f(v)).Float()) })
	default:
		return t.key(func(v T) []byte { return terminated([]byte(reflect.ValueOf(f(v)).String())) })
	}
}

// Rev adds a key like Cmp that orders values by f descending. only the order of this key is inverted, the
// position of the key in the chain is unchanged: ties still fall through to the next key.
func (t C[T]) Rev[R Ordered](f func(v T) R) C[T] {
	c := t.Cmp(f)

	// every encoding is prefix free, so inverting each byte inverts the comparison without bleeding into the next key.
	last := c.keys[len(c.keys)-1]
	c.keys[len(c.keys)-1] = func(v T) []byte {
		b := last(v)
		for i := range b {
			b[i] = ^b[i]
		}

		return b
	}

	return c
}

// int64Key encodes v so unsigned byte order matches signed numeric order.
func int64Key(v int64) []byte {
	return binary.BigEndian.AppendUint64(nil, uint64(v)^(1<<63))
}

// float64Key encodes v so unsigned byte order matches cmp.Compare.
func float64Key(v float64) []byte {
	switch {
	case v != v:
		// NaN orders before everything, including -Inf whose encoding is 0x000FFFFFFFFFFFFF.
		return binary.BigEndian.AppendUint64(nil, 0)
	case v == 0:
		// -0 and +0 are equal.
		v = 0
	}

	bits := math.Float64bits(v)
	if bits>>63 == 1 {
		bits = ^bits
	} else {
		bits |= 1 << 63
	}

	return binary.BigEndian.AppendUint64(nil, bits)
}

// terminated so a shorter value orders before its extensions without bleeding into the next key,
// zero bytes are escaped so the terminator is unambiguous.
func terminated(b []byte) []byte {
	out := make([]byte, 0, len(b)+2)
	for _, c := range b {
		if c == 0 {
			out = append(out, 0, 0xFF)
			continue
		}
		out = append(out, c)
	}

	return append(out, 0, 1)
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
