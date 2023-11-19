//go:build !wasm

package ffiexec

import (
	"unsafe"

	"github.com/james-lawrence/eg/interp/runtime/wasi/ffierrors"
)

func command(
	deadline int64, // context.Context
	dirptr unsafe.Pointer, dirlen uint32, // string
	envptr unsafe.Pointer, envsize, envlen uint32, // []string
	commandptr unsafe.Pointer, commandlen uint32, // string
	argsptr unsafe.Pointer, argssize, argslen uint32, // []string
) uint32 {
	return ffierrors.ErrNotImplemented
}
