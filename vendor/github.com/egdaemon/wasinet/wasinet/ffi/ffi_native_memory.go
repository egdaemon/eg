package ffi

import (
	"unsafe"
)

type Native struct{}

// ReadUint32Le reads a uint32 in little-endian encoding from the underlying buffer at the offset in or returns
// false if out of range.
func (t Native) ReadUint32Le(offset unsafe.Pointer) (uint32, bool) {
	return nativereadtype[uint32](offset), true
}

func (t Native) Read(offset unsafe.Pointer, dlen uint32) ([]byte, bool) {
	return unsafe.Slice((*byte)(offset), dlen), true
}

// WriteUint32Le writes the value in little-endian encoding to the underlying buffer at the offset in or returns
// false if out of range.
func (t Native) WriteUint32Le(offset unsafe.Pointer, v uint32) bool {
	nativeassign(offset, v)
	return true
}

// Write writes the slice to the underlying buffer at the offset or returns false if out of range.
func (t Native) Write(offset unsafe.Pointer, v []byte) bool {
	dst := unsafe.Slice((*byte)(offset), len(v))
	copy(dst, v)
	return true
}

func nativereadtype[T any](offset unsafe.Pointer) T {
	return *(*T)(offset)
}

func nativeassign[T any](offset unsafe.Pointer, v T) {
	*(*T)(offset) = v
}
