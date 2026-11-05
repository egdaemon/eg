//go:build !wasm

package ffiwasi

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

func start(
	idptr unsafe.Pointer, idlen uint32, // string
	dirptr unsafe.Pointer, dirlen uint32, // string
	envptr unsafe.Pointer, envlen, envsize uint32, // []string
	nameptr unsafe.Pointer, namelen uint32, // string
	argsptr unsafe.Pointer, argslen, argssize uint32, // []string
) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}

func wait(
	idptr unsafe.Pointer, idlen uint32, // string
) (exit uint32) {
	return ffierrors.ErrNotImplemented
}
