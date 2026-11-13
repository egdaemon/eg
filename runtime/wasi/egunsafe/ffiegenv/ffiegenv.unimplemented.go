//go:build !wasm

package ffiegenv

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

func environ(
	deadline int64, // context.Context
	bufptr unsafe.Pointer, bufcap uint32, // return []byte
	lenptr unsafe.Pointer, // return uint32
) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}
