//go:build !wasm

package fficoreutils

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

func run(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // string
	argsptr unsafe.Pointer, argslen, argssize uint32, // []string
) uint32 {
	return ffierrors.ErrNotImplemented
}

func start(
	deadline int64, // context.Context
	idptr unsafe.Pointer, idlen uint32, // string
	dirptr unsafe.Pointer, dirlen uint32, // string
	nameptr unsafe.Pointer, namelen uint32, // string
	argsptr unsafe.Pointer, argslen, argssize uint32, // []string
) uint32 {
	return ffierrors.ErrNotImplemented
}

func wait(
	idptr unsafe.Pointer, idlen uint32, // string
) uint32 {
	return ffierrors.ErrNotImplemented
}
