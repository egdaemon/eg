//go:build !wasm

package ffimetric

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

func record(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // metric name
	payload unsafe.Pointer, payloadlen uint32, // json payload
) uint32 {
	return ffierrors.ErrNotImplemented
}
