//go:build wasm

package ffigraph

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

// //go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/graph.Analysing
// func analysing() uint32

func analysing() uint32 {
	return ffierrors.ErrNotImplemented
}

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/graph.Push
func push(pid unsafe.Pointer, pidlen uint32, id unsafe.Pointer, idlen uint32) uint32

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/graph.Pop
func pop(pid unsafe.Pointer, pidlen uint32, id unsafe.Pointer, idlen uint32) uint32
