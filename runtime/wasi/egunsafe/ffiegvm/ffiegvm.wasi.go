//go:build wasm

package ffiegvm

import "unsafe"

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffiegvm.Run
func run(
	deadline int64, // context.Context
	locationptr unsafe.Pointer, locationlen uint32, // string
	tocptr unsafe.Pointer, toclen uint32, // string
	modulepathptr unsafe.Pointer, modulepathlen uint32, // string
	environptr unsafe.Pointer, environlen uint32, environsize uint32, // []string
) uint32

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffiegvm.Execveatinit
func Execveatinit(
	id uint64, // proc/{id}
) uint32

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffiegvm.Execveatwait
func Execveatwait(
	id uint64, // proc/{id}
) uint32
