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

func clone(
	deadline int64, // context.Context
	uriptr unsafe.Pointer, urilen uint32, // string
	remoteptr unsafe.Pointer, remotelen uint32, // string
	treeishptr unsafe.Pointer, treeishlen uint32, // string
) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}

func clone2(
	deadline int64, // context.Context
	uriptr unsafe.Pointer, urilen uint32, // string
	remoteptr unsafe.Pointer, remotelen uint32, // string
	treeishptr unsafe.Pointer, treeishlen uint32, // string
	envptr unsafe.Pointer, envsize, envlen uint32, // []string
) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}
