package ffiguest

import (
	"context"
	"math"
	"unsafe"

	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffierrors"
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

func ContextDeadline(ctx context.Context) int64 {
	if ts, ok := ctx.Deadline(); ok {
		return ts.UnixMicro()
	}

	return math.MaxInt64
}
