//go:build wasm

package ffigit

import (
	"unsafe"
)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Commitish
func commitish(
	deadline int64, // context.Context
	treeishptr unsafe.Pointer, treeishlen uint32, // string
	commitptr unsafe.Pointer, commitlen uint32, // return string
) (errcode uint32)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Clone
func clone(
	deadline int64, // context.Context
	uriptr unsafe.Pointer, urilen uint32, // string
	remoteptr unsafe.Pointer, remotelen uint32, // string
	treeishptr unsafe.Pointer, treeishlen uint32, // string
) (errcode uint32)
