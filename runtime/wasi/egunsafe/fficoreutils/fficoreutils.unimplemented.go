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
