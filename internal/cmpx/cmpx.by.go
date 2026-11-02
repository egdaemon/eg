package cmpx

import (
	"reflect"
	"time"
	"unsafe"
)

// S is a C that can also key on the fields of a struct with By, created by For and From. it embeds C, so
// Compare and With are available, the key methods are repeated so a chain stays an S.
type S[T any] struct {
	C[T]

	// the sample the field pointers given to By are relative to.
	base unsafe.Pointer
	elem reflect.Type
}

// For starts a comparison of structs of type T. base is an addressable sample, the fields given to By must
// be fields of base, for example:
//
//	var z Foo
//	cmpx.For(&z).By(&z.Weight)
//
// panics when T is not a struct or base is nil.
func For[T any](base *T) S[T] {
	if reflect.TypeFor[T]().Kind() != reflect.Struct {
		panic("cmpx: For requires a struct")
	}

	if base == nil {
		panic("cmpx: For requires a non nil sample")
	}

	return S[T]{base: unsafe.Pointer(base), elem: reflect.TypeFor[T]()}
}

// From starts a comparison with v as the left hand side, see With, or use Compare.
// when v is a non nil pointer to a struct it is also the sample for By, comparing pointers to the struct:
//
//	var z Foo
//	cmpx.From(&z).By(&z.Weight)
func From[T any](v T) S[T] {
	s := S[T]{C: C[T]{v: v}}

	if rt := reflect.TypeFor[T](); rt.Kind() == reflect.Pointer && rt.Elem().Kind() == reflect.Struct {
		if rv := reflect.ValueOf(v); !rv.IsNil() {
			s.base, s.elem = rv.UnsafePointer(), rt.Elem()
		}
	}

	return s
}

// Cmp see C.Cmp.
func (t S[T]) Cmp[R Ordered](f func(v T) R) S[T] {
	t.C = t.C.Cmp(f)
	return t
}

// Rev see C.Rev.
func (t S[T]) Rev[R Ordered](f func(v T) R) S[T] {
	t.C = t.C.Rev(f)
	return t
}

// Bytes see C.Bytes.
func (t S[T]) Bytes(f func(v T) []byte) S[T] {
	t.C = t.C.Bytes(f)
	return t
}

// Time see C.Time.
func (t S[T]) Time(f func(v T) time.Time) S[T] {
	t.C = t.C.Time(f)
	return t
}

// By adds a key that orders values by the field p points to, ascending. p must point to a field of the
// sample given to For or From, for example:
//
//	var z Foo
//	cmpx.From(&z).By(&z.Weight).By(&z.Name)
//
// nested fields are supported, fields reached through a pointer are not. a nil pointer orders before every other value.
// panics when there is no sample, for example From with a value that is not a non nil pointer to a struct, or p is
// not a field of type R of the sample.
func (t S[T]) By[R Ordered](p *R) S[T] {
	if t.base == nil {
		panic("cmpx: By requires a sample, create the comparison with For(&z) or From(&z) where z is a struct")
	}

	var (
		rt    = reflect.TypeFor[R]()
		size  = t.elem.Size()
		off   = uintptr(unsafe.Pointer(p)) - uintptr(t.base)
		isptr = reflect.TypeFor[T]().Kind() == reflect.Pointer
	)

	if rt.Size() > size || off > size-rt.Size() {
		panic("cmpx: By requires a field of the sample")
	}

	if !fieldAt(t.elem, off, rt) {
		panic("cmpx: By requires a field of the sample with a matching type")
	}

	ptr := func(v T) unsafe.Pointer {
		if p := unsafe.Pointer(&v); !isptr {
			return p
		} else {
			return *(*unsafe.Pointer)(p)
		}
	}

	t.C = t.C.Cmp(func(v T) R { return *(*R)(unsafe.Add(ptr(v), off)) })

	// a nil pointer has no field to read, it orders before every other value.
	last := t.C.keys[len(t.C.keys)-1]
	t.C.keys[len(t.C.keys)-1] = func(v T) []byte {
		if ptr(v) == nil {
			return nil
		}

		return last(v)
	}

	return t
}

// fieldAt reports whether typ has a field of type want at the offset off.
func fieldAt(typ reflect.Type, off uintptr, want reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Struct:
		for i := range typ.NumField() {
			f := typ.Field(i)
			if off < f.Offset || off >= f.Offset+f.Type.Size() {
				continue
			}

			if fieldAt(f.Type, off-f.Offset, want) {
				return true
			}
		}

		return false
	case reflect.Array:
		if size := typ.Elem().Size(); size > 0 {
			return fieldAt(typ.Elem(), off%size, want)
		}

		return false
	default:
		return off == 0 && typ == want
	}
}
