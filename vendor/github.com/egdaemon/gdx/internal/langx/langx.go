// Package langx provides small utility functions to extend the standard golang language.
package langx

import "reflect"

// Autoptr converts a value into a pointer
func Autoptr[T any](a T) *T {
	return &a
}

// Autoderef safely converts a pointer to its value, uses the zero value for nil.
func Autoderef[T any](a *T) (zero T) {
	if a == nil {
		return zero
	}

	return *a
}

func Zero[T any](a *T) (zero T) {
	if a == nil {
		return zero
	}

	return *a
}

func Clone[T any, Y ~func(*T)](v T, options ...Y) T {
	dup := v
	for _, opt := range options {
		opt(&dup)
	}

	return dup
}

func Compose[T any, Y ~func(*T)](options ...Y) Y {
	return func(v *T) {
		for _, opt := range options {
			opt(v)
		}
	}
}

func FirstNonZero[T comparable](s ...T) T {
	var (
		x T
	)

	for _, v := range s {
		if v == x {
			continue
		}

		return v
	}

	return x
}

func FirstNonNil[T any](s ...T) T {
	var zero T

	for _, v := range s {
		if !isNil(v) {
			return v
		}
	}

	return zero
}

// isNil reports whether a value is nil using reflection.
// Zero values (e.g. int(0)) are not considered nil.
func isNil[T any](v T) bool {
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Interface, reflect.Func, reflect.UnsafePointer:
		return r.IsNil()
	default:
		return false
	}
}
