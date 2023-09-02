package ffigraph

import (
	"unsafe"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiguest"
)

//go:wasmimport env github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Analysing
func analysing() uint32

//go:wasmimport env github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Push
func push(id unsafe.Pointer, idlen uint32) uint32

//go:wasmimport env github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Pop
func pop(id unsafe.Pointer, idlen uint32) uint32

func Analysing() bool {
	return analysing() == 0
}

type node interface {
	ID() string
}

func WrapErr(op node, fn func() error) error {
	push(ffiguest.String(op.ID()))
	defer pop(ffiguest.String(op.ID()))
	return fn()
}
