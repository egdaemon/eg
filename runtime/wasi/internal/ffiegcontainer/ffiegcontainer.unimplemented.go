//go:build !wasm

package ffiegcontainer

import (
	"unsafe"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffierrors"
)

func pull(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // string
	argsptr unsafe.Pointer, argssize, argslen uint32, // []string
) uint32 {
	return ffierrors.ErrNotImplemented
}

func build(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // string
	definitionptr unsafe.Pointer, definitionlen uint32, // string
	argsptr unsafe.Pointer, argslen uint32, argssize uint32, // []string
) uint32 {
	return ffierrors.ErrNotImplemented
}

func run(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // string
	modulepathptr unsafe.Pointer, modulepathlen uint32, // string
	cmdptr unsafe.Pointer, cmdsize, cmdlen uint32, // []string
	argsptr unsafe.Pointer, argslen uint32, argssize uint32, // []string
) uint32 {
	return ffierrors.ErrNotImplemented
}

func module(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // string
	modulepathptr unsafe.Pointer, modulepathlen uint32, // string
	argsptr unsafe.Pointer, argslen uint32, argssize uint32, // []string
) uint32 {
	return ffierrors.ErrNotImplemented
}
