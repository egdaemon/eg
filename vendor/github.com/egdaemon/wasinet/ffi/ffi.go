package ffi

import (
	"context"
	"syscall"
	"time"
	"unsafe"
)

type Memory interface {
	// // ReadUint32Le reads a uint32 in little-endian encoding from the underlying buffer at the offset in or returns
	// // false if out of range.
	ReadUint32Le(offset uint32) (uint32, bool)

	Read(offset, byteCount uint32) ([]byte, bool)

	// WriteUint32Le writes the value in little-endian encoding to the underlying buffer at the offset in or returns
	// false if out of range.
	WriteUint32Le(offset, v uint32) bool

	// Write writes the slice to the underlying buffer at the offset or returns false if out of range.
	Write(offset uint32, v []byte) bool
}

func ReadString(m Memory, offset uintptr, len uint32) (string, error) {
	var (
		ok   bool
		data []byte
	)

	if data, ok = m.Read(uint32(offset), len); !ok {
		return "", syscall.EFAULT
	}

	return string(data), nil
}

func ReadStringArray(m Memory, offset uint32, length uint32, argssize uint32) (args []string, err error) {
	args = make([]string, 0, length)

	for offset, i := offset, uint32(0); i < length*2; offset, i = offset+(2*argssize), i+2 {
		var (
			data []byte
		)

		if data, err = ReadArrayElement(m, offset, argssize); err != nil {
			return nil, err
		}

		args = append(args, string(data))
	}

	return args, nil
}

func ReadArrayElement(m Memory, offset, len uint32) (data []byte, err error) {
	var (
		ok            bool
		eoffset, elen uint32
	)

	if eoffset, ok = m.ReadUint32Le(offset); !ok {
		return nil, syscall.EFAULT
	}

	if elen, ok = m.ReadUint32Le(offset + len); !ok {
		return nil, syscall.EFAULT
	}

	if data, ok = m.Read(eoffset, elen); !ok {
		return nil, syscall.EFAULT
	}

	return data, nil
}

func ReadMicroDeadline(ctx context.Context, deadline int64) (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, time.UnixMicro(deadline))
}

func BytesRead(m Memory, offset uintptr, len uint32) (data []byte, err error) {
	var (
		ok bool
	)

	if data, ok = m.Read(uint32(offset), len); !ok {
		return nil, syscall.EFAULT
	}

	return data, nil
}

func RawRead[T any](m Memory, ptr uintptr, dlen uint32) (zero *T, err error) {
	if binary, ok := m.Read(uint32(ptr), dlen); !ok {
		return zero, syscall.EFAULT
	} else {
		return (*T)(unsafe.Pointer(unsafe.SliceData(binary))), nil
	}
}

func Uint32Read(m Memory, ptr uintptr, dlen uint32) (uint32, error) {
	if v, ok := m.ReadUint32Le(uint32(ptr)); !ok {
		return 0, syscall.EFAULT
	} else {
		return v, nil
	}
}

func Uint32Write(m Memory, dst uintptr, v uint32) error {
	if !m.WriteUint32Le(uint32(dst), v) {
		return syscall.EFAULT
	}

	return nil
}

func RawWrite[T any](m Memory, v T, dst uintptr, dlen uint32) error {
	if !m.Write(uint32(dst), unsafe.Slice((*byte)(unsafe.Pointer(&v)), unsafe.Sizeof(v))) {
		return syscall.EFAULT
	}

	return nil
}

func BytesWrite(m Memory, v []byte, dst uintptr, dlen uint32) error {
	if !m.Write(uint32(dst), v) {
		return syscall.EFAULT
	}

	return nil
}
