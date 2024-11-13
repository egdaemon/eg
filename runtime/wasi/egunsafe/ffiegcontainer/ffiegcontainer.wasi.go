//go:build wasm

package ffiegcontainer

import "unsafe"

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Pull
func pull(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // string
	argsptr unsafe.Pointer, argssize, argslen uint32, // []string
) uint32

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Build
func build(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // string
	definitionptr unsafe.Pointer, definitionlen uint32, // string
	argsptr unsafe.Pointer, argssize, argslen uint32, // []string
) uint32

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Run
func run(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // string
	modulepathptr unsafe.Pointer, modulepathlen uint32, // string
	cmdptr unsafe.Pointer, cmdsize, cmdlen uint32, // []string
	argsptr unsafe.Pointer, argssize, argslen uint32, // []string
) uint32

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffiegcontainer.Module
func module(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // string
	modulepathptr unsafe.Pointer, modulepathlen uint32, // string
	argsptr unsafe.Pointer, argssize, argslen uint32, // []string
) uint32
