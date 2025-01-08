//go:build !wasm

package ffigraph

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

func _recordevt(deadline int64, evt unsafe.Pointer, evtlen uint32) uint32 {
	return ffierrors.ErrNotImplemented
}
