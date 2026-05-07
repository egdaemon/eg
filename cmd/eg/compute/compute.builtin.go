package compute

import (
	"context"
	"embed"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdplete"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/workspaces"
	"github.com/posener/complete"
)

type cmdbuiltin struct {
	Upload builtinUpload `cmd:"" help:"upload and run a builtin workload"`
	Local  builtinLocal  `cmd:"" help:"upload and run a builtin workload"`
}

//go:embed .builtin
var embeddedbuiltin embed.FS

func NewBuiltinPredictor(ctx context.Context) complete.Predictor {
	current := md5x.FormatString(md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())))
	dir := userx.DefaultCacheDirectory("builtin", current)

	return cmdplete.InitializingPrediction(func() error {
		root := filepath.Dir(dir)

		if err := fsx.MkDirs(0700, root); err != nil {
			return err
		}

		for path := range fsx.KeepNewestN(3, fsx.Find(root, fsx.Levels(1))).Each(ctx) {
			errorsx.Log(errorsx.Wrapf(os.RemoveAll(path), "builtin cache cleanup: %s", path))
		}

		if err := fsx.DirExists(dir); err == nil {
			return nil
		}

		if err := workspaces.PrepareBuiltin(ctx, dir, embeddedbuiltin); err != nil {
			return err
		}

		if err := compile.InitGolang(ctx, dir, cmdopts.ModPath()); err != nil {
			return errorsx.Wrap(err, "failed to init go mod")
		}

		if err := compile.InitGolangTidy(ctx, dir); err != nil {
			return errorsx.Wrap(err, "failed to tidy packages")
		}

		return nil
	}, cmdplete.NewWorkload(dir))
}
