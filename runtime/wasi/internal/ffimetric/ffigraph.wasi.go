//go:build wasm

package ffimetric

import (
	"unsafe"
)

//go:wasmimport env github.com/egdaemon/eg/runtime/wasi/runtime/metrics.Record
func record(
	deadline int64, // context.Context
	nameptr unsafe.Pointer, namelen uint32, // metric name
	payload unsafe.Pointer, payloadlen uint32, // json payload
) uint32
