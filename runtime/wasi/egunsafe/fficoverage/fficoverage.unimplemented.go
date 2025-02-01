//go:build !wasm

package fficoverage

import (
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

func record(
	deadline int64, // context.Context
	payload unsafe.Pointer, payloadlen uint32, // json payload
) uint32 {
	return ffierrors.ErrNotImplemented
}
