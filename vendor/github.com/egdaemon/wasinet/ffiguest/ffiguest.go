package ffiguest

import (
	"context"
	"encoding/binary"
	"math"
	"unsafe"

	"github.com/egdaemon/wasinet/ffierrors"
	"github.com/egdaemon/wasinet/internal/errorsx"
)

func Error(code uint32, msg error) error {
	if code == 0 {
		return nil
	}

	cause := errorsx.Wrapf(msg, "wasi host error: %d", code)
	switch code {
	case ffierrors.ErrUnrecoverable:
		return errorsx.NewUnrecoverable(cause)
	default:
		return cause
	}
}

func String(s string) (unsafe.Pointer, uint32) {
	return unsafe.Pointer(unsafe.StringData(s)), uint32(len(s))
}

func StringRead(dptr unsafe.Pointer, dlen uint32) string {
	return unsafe.String((*byte)(dptr), dlen)
}

func StringArray(a ...string) (unsafe.Pointer, uint32, uint32) {
	return unsafe.Pointer(unsafe.SliceData(a)), uint32(len(a)), uint32(unsafe.Sizeof(&a))
}

func BytesResult(d []byte, reslengthdst *uint32) (unsafe.Pointer, uint32, unsafe.Pointer) {
	return unsafe.Pointer(unsafe.SliceData(d)), uint32(len(d)), unsafe.Pointer(reslengthdst)
}

func Bytes(d []byte) (unsafe.Pointer, uintptr) {
	return unsafe.Pointer(unsafe.SliceData(d)), uintptr(len(d))
}

func BytesRead(dptr unsafe.Pointer, dlen uintptr) []byte {
	return unsafe.Slice((*byte)(dptr), dlen)
}

func ContextDeadline(ctx context.Context) int64 {
	if ts, ok := ctx.Deadline(); ok {
		return ts.UnixMicro()
	}

	return math.MaxInt64
}

func Uint32(dst *uint32) unsafe.Pointer {
	return unsafe.Pointer(dst)
}

func WriteInt32(dst unsafe.Pointer, src int32) {
	*(*int32)(dst) = src
}

func ReadUint32(ptr unsafe.Pointer, length uintptr) uint32 {
	return binary.LittleEndian.Uint32(BytesRead(ptr, length))
}

func WriteUint32(dst unsafe.Pointer, src uint32) {
	*(*uint32)(dst) = src
}

func WriteRaw[T any](dst unsafe.Pointer, src T) {
	*(*T)(dst) = src
}

func Raw[T any](s *T) (unsafe.Pointer, uintptr) {
	return unsafe.Pointer(s), unsafe.Sizeof(*s)
}

func RawRead[T any](ptr unsafe.Pointer, dlen uintptr) *T {
	return (*T)(ptr)
}
