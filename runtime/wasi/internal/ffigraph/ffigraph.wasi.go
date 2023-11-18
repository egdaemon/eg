//go:build wasm

package ffigraph

import (
	"unsafe"
)

//go:wasmimport env github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Analysing
func analysing() uint32

//go:wasmimport env github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Push
func push(pid unsafe.Pointer, pidlen uint32, id unsafe.Pointer, idlen uint32) uint32

//go:wasmimport env github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Pop
func pop(pid unsafe.Pointer, pidlen uint32, id unsafe.Pointer, idlen uint32) uint32
