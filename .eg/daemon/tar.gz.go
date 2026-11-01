package daemon

import (
	"context"

	debeg "eg/compute/debuild/eg"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func Tarball(ctx context.Context, op eg.Op) error {
	return eg.Perform(
		ctx,
		debeg.Prepare,
		eg.Module(
			ctx,
			debeg.Runner(),
			eg.Sequential(
				debeg.Build,
				debeg.Upload,
				shell.Op(
					shell.Newf("cp %s/*.deb %s", egenv.DirectoryTemp("deb.eg"), egenv.DirectoryWorkload(".dist")),
				),
			),
		),
	)
}
