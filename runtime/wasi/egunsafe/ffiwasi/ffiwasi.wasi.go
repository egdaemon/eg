//go:build wasm

package ffiwasi

import (
	"unsafe"
)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffiwasi.Start
func start(
	idptr unsafe.Pointer, idlen uint32, // string
	dirptr unsafe.Pointer, dirlen uint32, // string
	envptr unsafe.Pointer, envlen, envsize uint32, // []string
	nameptr unsafe.Pointer, namelen uint32, // string
	argsptr unsafe.Pointer, argslen, argssize uint32, // []string
) (errcode uint32)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffiwasi.Wait
func wait(
	idptr unsafe.Pointer, idlen uint32, // string
) (exit uint32)
