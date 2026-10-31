// Package ffipatch has helpers for producing patches of a workload's changes.
// it is not part of the stable api.
package ffipatch

import (
	"context"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

// Patch generates a patch of the current working tree changes, including new files, and writes it to dest.
func Patch(dest string) eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		return shell.Run(
			ctx,
			shell.Runtime().Directory(egenv.DirectoryWorking()).Newf("git add -N . && git diff HEAD > %s", dest),
		)
	}
}
