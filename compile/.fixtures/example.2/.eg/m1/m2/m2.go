package m2

import (
	"context"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func Debug(ctx context.Context, _ eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("echo m2"),
	)
}

func Root(ctx context.Context, _ eg.Op) error {
	c1 := eg.Container("egmeta.ubuntu.24.10")
	return eg.Perform(
		ctx,
		eg.Module(ctx, c1, Debug),
	)
}
