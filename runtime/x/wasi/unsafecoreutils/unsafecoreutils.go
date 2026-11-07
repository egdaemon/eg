// Package unsafecoreutils runs uutils coreutils from within a guest module. The utilities execute
// in a sandboxed wasm instance provided by the host.
package unsafecoreutils

//go:generate go run ./internal/generate -out unsafecoreutils.gen.go
