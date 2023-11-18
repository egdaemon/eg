//go:build wasm

package ffigit

import "unsafe"

//go:wasmimport env github.com/james-lawrence/eg/runtime/wasi/runtime/ffigit.Commitish
func commitish(
	deadline int64, // context.Context
	treeishptr unsafe.Pointer, treeishlen uint32, // string
	commitptr unsafe.Pointer, commitlen uint32, // return string
) (errcode uint32)
