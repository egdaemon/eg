package workspaces

import (
	"context"
	"io/fs"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
)

func PrepareBuiltin(ctx context.Context, root string, archive fs.FS) error {
	if err := fsx.MkDirs(0700, root); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}

	if err := fsx.CloneTree(ctx, root, ".builtin", archive); err != nil {
		return errorsx.Wrap(err, "unable to clone tree")
	}

	return nil
}
