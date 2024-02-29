//go:build !wasm

package ffigit

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

func commitish(
	deadline int64, // context.Context
	treeishptr unsafe.Pointer, treeishlen uint32, // string
	commitptr unsafe.Pointer, commitlen uint32, // return string
) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}
