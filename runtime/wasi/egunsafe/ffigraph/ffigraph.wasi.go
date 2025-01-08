//go:build wasm

package ffigraph

import (
	"unsafe"
)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/graph.Trace
func _recordevt(deadline int64, evt unsafe.Pointer, evtlen uint32) uint32
