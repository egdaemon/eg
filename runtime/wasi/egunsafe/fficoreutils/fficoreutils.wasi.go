//go:build wasm

package fficoreutils

import (
	"unsafe"
)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/fficoreutils.Run
func run(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // string
	argsptr unsafe.Pointer, argslen, argssize uint32, // []string
) uint32

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/fficoreutils.Start
func start(
	deadline int64, // context.Context
	idptr unsafe.Pointer, idlen uint32, // string
	dirptr unsafe.Pointer, dirlen uint32, // string
	nameptr unsafe.Pointer, namelen uint32, // string
	argsptr unsafe.Pointer, argslen, argssize uint32, // []string
) uint32

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/fficoreutils.Wait
func wait(
	idptr unsafe.Pointer, idlen uint32, // string
) uint32
