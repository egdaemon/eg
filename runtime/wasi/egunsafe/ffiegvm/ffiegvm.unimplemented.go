//go:build !wasm

package ffiegvm

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

func run(
	deadline int64, // context.Context
	locationptr unsafe.Pointer, locationlen uint32, // string
	tocptr unsafe.Pointer, toclen uint32, // string
	modulepathptr unsafe.Pointer, modulepathlen uint32, // string
	environptr unsafe.Pointer, environlen uint32, environsize uint32, // []string
) uint32 {
	return ffierrors.ErrNotImplemented
}

func Execveatinit(
	id uint64, // proc/{id}
) uint32 {
	return ffierrors.ErrNotImplemented
}

func Execveatwait(
	id uint64, // proc/{id}
) uint32 {
	return ffierrors.ErrNotImplemented
}
