// Package unsafecoreutils runs uutils coreutils from within a guest module. The utilities execute
// in the egcoreutils reactor module, loaded alongside the guest.
package unsafecoreutils

import (
	"context"
	"os"

	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoreutils"
)

//go:generate go run ./internal/generate -out unsafecoreutils.gen.go

// run executes the utility with this module's stdio.
func run(ctx context.Context, name string, args ...string) error {
	cmd := fficoreutils.Command(ctx, name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
