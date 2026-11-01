package ffiguest

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"unsafe"

	"github.com/james-lawrence/genieql/internal/errorsx"
	"github.com/james-lawrence/genieql/internal/wasix/ffierrors"
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

func File(o *os.File) (int64, unsafe.Pointer, uint32) {
	return int64(o.Fd()), unsafe.Pointer(unsafe.StringData(o.Name())), uint32(len(o.Name()))
}

func String(s string) (unsafe.Pointer, uint32) {
	return unsafe.Pointer(unsafe.StringData(s)), uint32(len(s))
}

func StringArray(a ...string) (unsafe.Pointer, uint32, uint32) {
	return unsafe.Pointer(unsafe.SliceData(a)), uint32(len(a)), uint32(unsafe.Sizeof(&a))
}

func Bytes(d []byte) (unsafe.Pointer, uint32) {
	return unsafe.Pointer(unsafe.SliceData(d)), uint32(len(d))
}

func ByteBuffer(buf []byte) (buflen uint32, dptr unsafe.Pointer, dlen uint32) {
	return uint32(cap(buf)), unsafe.Pointer(unsafe.SliceData(buf)), dlen
}

func ByteBufferRead(dptr unsafe.Pointer, dlen uint32) []byte {
	return unsafe.Slice((*byte)(dptr), dlen)
}

func JSON(d any) (unsafe.Pointer, uint32, error) {
	encoded, err := json.Marshal(d)
	if err != nil {
		return nil, 0, err
	}

	ptr, l := Bytes(encoded)
	return ptr, l, nil
}

// Extracts the deadline for the context and converts it to a unix micro timestamp.
// if there is no deadline than math.MaxInt64 is return which is effectively saying
// there is no timeout.
func ContextMicroDeadline(ctx context.Context) int64 {
	if ts, ok := ctx.Deadline(); ok {
		return ts.UnixMicro()
	}

	return math.MaxInt64
}

func Bool(b byte) bool {
	switch b {
	case 0:
		return false
	default:
		return true
	}
}
