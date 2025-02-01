//go:build wasm

package fficoverage

import (
	"unsafe"
)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/coverage.Report
func record(
	deadline int64, // context.Context
	payload unsafe.Pointer, payloadlen uint32, // json payload
) uint32
