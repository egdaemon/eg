package ffi

import (
	"syscall"
	"unsafe"
)

type Memory interface {
	Read(offset unsafe.Pointer, byteCount uint32) ([]byte, bool)

	// Write writes the slice to the underlying buffer at the offset or returns false if out of range.
	Write(offset unsafe.Pointer, v []byte) bool

	// ReadUint32Le reads a uint32 in little-endian encoding from the underlying buffer at the offset in or returns
	// false if out of range.
	ReadUint32Le(offset unsafe.Pointer) (uint32, bool)

	// WriteUint32Le writes the value in little-endian encoding to the underlying buffer at the offset in or returns
	// false if out of range.
	WriteUint32Le(offset unsafe.Pointer, v uint32) bool
}

func SliceRead[T any](m Memory, offset unsafe.Pointer, dlen uint32) (zero []T, err error) {
	if binary, ok := m.Read(offset, dlen); !ok {
		return zero, syscall.EFAULT
	} else {
		return *(*[]T)(unsafe.Pointer(&binary)), nil
	}
}

func BytesRead(m Memory, offset unsafe.Pointer, len uint32) (data []byte, err error) {
	var (
		ok bool
	)

	if data, ok = m.Read(offset, len); !ok {
		return nil, syscall.EFAULT
	}

	return data, nil
}

func RawRead(m Memory, dm Memory, dst unsafe.Pointer, ptr unsafe.Pointer, dlen uint32) (err error) {
	if binary, ok := m.Read(ptr, dlen); !ok {
		return syscall.EFAULT
	} else {
		if !dm.Write(dst, binary) {
			return syscall.EFAULT
		}
		return nil
	}
}

func RawWrite[T any](m Memory, v *T, dst unsafe.Pointer, dlen uint32) error {
	sz := unsafe.Sizeof(*v)
	ptr := (*byte)(unsafe.Pointer(v))
	bytes := unsafe.Slice(ptr, sz)
	if !m.Write(dst, bytes) {
		return syscall.EFAULT
	}
	return nil
}

func Uint32ReadNative(ptr unsafe.Pointer, len uint32) (uint32, error) {
	return Uint32Read(Native{}, ptr, len)
}

func Uint32Read(m Memory, ptr unsafe.Pointer, dlen uint32) (uint32, error) {
	if v, ok := m.ReadUint32Le(ptr); !ok {
		return 0, syscall.EFAULT
	} else {
		return v, nil
	}
}

func Uint32Write(m Memory, dst unsafe.Pointer, v uint32) error {
	if !m.WriteUint32Le(dst, v) {
		return syscall.EFAULT
	}

	return nil
}

func BytesWrite(m Memory, v []byte, dst unsafe.Pointer, dlen uint32) error {
	if !m.Write(dst, v) {
		return syscall.EFAULT
	}

	return nil
}

func Int32Write(m Memory, dst unsafe.Pointer, v int32) error {
	if !m.WriteUint32Le(dst, uint32(v)) {
		return syscall.EFAULT
	}

	return nil
}

func UnsafeClone[T any](ptr unsafe.Pointer) T {
	return *(*T)(ptr)
}

type Vector struct {
	Offset unsafe.Pointer
	Length uint32
}

func VectorSlice[T any](eles ...[]T) []Vector {
	iovsBuf := make([]Vector, 0, len(eles))
	for _, iov := range eles {
		iovsBuf = append(iovsBuf, Vector{
			Offset: unsafe.Pointer(unsafe.SliceData(iov)),
			Length: uint32(len(iov)),
		})
	}

	return iovsBuf
}

func VectorRead[T any](m Memory, eles ...Vector) ([][]T, error) {
	r := make([][]T, 0, len(eles))
	for _, v := range eles {
		if v2, err := SliceRead[T](m, unsafe.Pointer(v.Offset), v.Length); err != nil {
			return r, err
		} else {
			r = append(r, v2)
		}
	}

	return r, nil
}

func Pointer[T any](s *T) (unsafe.Pointer, uint32) {
	return unsafe.Pointer(s), uint32(unsafe.Sizeof(*s))
}

func StringReadNative(offset unsafe.Pointer, len uint32) (string, error) {
	return StringRead(Native{}, offset, len)
}

func StringRead(m Memory, offset unsafe.Pointer, len uint32) (string, error) {
	var (
		ok   bool
		data []byte
	)

	if data, ok = m.Read(offset, len); !ok {
		return "", syscall.EFAULT
	}

	return string(data), nil
}

func String(s string) (unsafe.Pointer, uint32) {
	return unsafe.Pointer(unsafe.StringData(s)), uint32(len(s))
}

func Slice[T any](d []T) (unsafe.Pointer, uint32) {
	return unsafe.Pointer(unsafe.SliceData(d)), uint32(len(d))
}
