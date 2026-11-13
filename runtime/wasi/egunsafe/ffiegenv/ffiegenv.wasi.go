//go:build wasm

package ffiegenv

import (
	"unsafe"
)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffiegenv.Environ
func environ(
	deadline int64, // context.Context
	bufptr unsafe.Pointer, bufcap uint32, // return []byte
	lenptr unsafe.Pointer, // return uint32
) (errcode uint32)
