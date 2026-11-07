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

//go:wasmimport egcoreutils execveatinit
func execveatinit(
	id uint64, // proc/{id}
) uint32

//go:wasmimport egcoreutils execveatwait
func execveatwait(
	id uint64, // proc/{id}
) uint32
