// Package unsafecoreutils runs uutils coreutils from within a guest module. The utilities execute
// in a sandboxed wasm instance provided by the host.
package unsafecoreutils

import (
	"context"

	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoreutils"
)

// ExitError is returned when a utility exits with a non zero code.
type ExitError = fficoreutils.ExitError

func Cat(ctx context.Context, args ...string) error   { return fficoreutils.Run(ctx, "cat", args...) }
func Echo(ctx context.Context, args ...string) error  { return fficoreutils.Run(ctx, "echo", args...) }
func Ls(ctx context.Context, args ...string) error    { return fficoreutils.Run(ctx, "ls", args...) }
func Nproc(ctx context.Context, args ...string) error { return fficoreutils.Run(ctx, "nproc", args...) }
