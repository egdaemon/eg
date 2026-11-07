//go:build wasm

package fficoreutils

//go:wasmimport egcoreutils execveatinit
func execveatinit(
	id uint64, // proc/{id}
) uint32

//go:wasmimport egcoreutils execveatwait
func execveatwait(
	id uint64, // proc/{id}
) uint32
