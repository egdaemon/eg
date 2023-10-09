package ffigraph

import (
	"unsafe"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffiguest"
)

//go:wasmimport env github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Analysing
func analysing() uint32

//go:wasmimport env github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Push
func push(pid unsafe.Pointer, pidlen uint32, id unsafe.Pointer, idlen uint32) uint32

//go:wasmimport env github.com/james-lawrence/eg/runtime/wasi/runtime/graph.Pop
func pop(pid unsafe.Pointer, pidlen uint32, id unsafe.Pointer, idlen uint32) uint32

func Analysing() bool {
	return analysing() == 0
}

type node interface {
	ID() string
}

func WrapErr(parent, op node, fn func() error) error {
	pid, plen := ffiguest.String("")
	if parent != nil {
		pid, plen = ffiguest.String(parent.ID())
	}

	oid, olen := ffiguest.String(op.ID())
	push(pid, plen, oid, olen)
	defer pop(pid, plen, oid, olen)

	return fn()
}
