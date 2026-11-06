// Package unsafecoreutils runs uutils coreutils from within a guest module. The utilities execute
// in a sandboxed wasm instance provided by the host.
package unsafecoreutils

import (
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoreutils"
)

// ExitError is returned when a utility exits with a non zero code.
type ExitError = fficoreutils.ExitError

//go:generate go run ./internal/generate -out unsafecoreutils.gen.go
