//go:build wasm

package ffigit

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Commitish
func commitish(
	deadline int64, // context.Context
	treeishptr unsafe.Pointer, treeishlen uint32, // string
	commitptr unsafe.Pointer, commitlen uint32, // return string
) (errcode uint32)

// clone is the original host function, it is no longer provided by the host. kept so the symbol exists.
func clone(
	deadline int64, // context.Context
	uriptr unsafe.Pointer, urilen uint32, // string
	remoteptr unsafe.Pointer, remotelen uint32, // string
	treeishptr unsafe.Pointer, treeishlen uint32, // string
) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.Bearer
func bearer(
	tokenptr unsafe.Pointer, tokenlen uint32,
) (errcode uint32)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/ffigit.CloneV3
func clone3(
	deadline int64, // context.Context
	dstptr unsafe.Pointer, dstlen uint32, // string
	uriptr unsafe.Pointer, urilen uint32, // string
	remoteptr unsafe.Pointer, remotelen uint32, // string
	treeishptr unsafe.Pointer, treeishlen uint32, // string
	envptr unsafe.Pointer, envsize, envlen uint32, // []string
) (errcode uint32)
