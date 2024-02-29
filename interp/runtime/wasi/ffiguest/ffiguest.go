package ffiguest

import (
	"context"
	"math"
	"unsafe"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
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

func Bytes(d []byte) (unsafe.Pointer, uint32) {
	return unsafe.Pointer(unsafe.SliceData(d)), uint32(len(d))
}

func BytesRead(dptr unsafe.Pointer, dlen uint32) []byte {
	return unsafe.Slice((*byte)(dptr), dlen)
}

func ContextDeadline(ctx context.Context) int64 {
	if ts, ok := ctx.Deadline(); ok {
		return ts.UnixMicro()
	}

	return math.MaxInt64
}
