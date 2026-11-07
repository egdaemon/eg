//go:build !wasm

package fficoreutils

import (
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
)

func execveatinit(
	id uint64, // proc/{id}
) uint32 {
	return ffierrors.ErrNotImplemented
}

func execveatwait(
	id uint64, // proc/{id}
) uint32 {
	return ffierrors.ErrNotImplemented
}
