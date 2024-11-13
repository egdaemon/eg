//go:build !wasm

package ffigraph

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

func analysing() uint32 {
	return ffierrors.ErrNotImplemented
}

func push(pid unsafe.Pointer, pidlen uint32, id unsafe.Pointer, idlen uint32) uint32 {
	return ffierrors.ErrNotImplemented
}

func pop(pid unsafe.Pointer, pidlen uint32, id unsafe.Pointer, idlen uint32) uint32 {
	return ffierrors.ErrNotImplemented
}
