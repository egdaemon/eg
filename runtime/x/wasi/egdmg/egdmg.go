package egdmg

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

type Option func(*Specification)

func OptionRuntime(s shell.Command) Option {
	return func(d *Specification) {
		d.runtime = s
	}
}

func OptionBuildDir(s string) Option {
	return func(d *Specification) {
		d.builddir = s
	}
}

func New(name string, options ...Option) Specification {
	return langx.Clone(Specification{
		name:     name,
		runtime:  shell.Runtime(),
		builddir: egenv.EphemeralDirectory(),
	}, options...)
}

type Specification struct {
	name       string
	outputpath string
	builddir   string
	runtime    shell.Command
}

func Build(b Specification, archive fs.FS) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		if err := os.Symlink("/Applications", filepath.Join(b.builddir, "Applications")); err != nil {
			return err
		}

		root := fmt.Sprintf("%s.app", b.name)
		if err := egfs.CloneFS(ctx, filepath.Join(b.builddir, root), ".", archive); err != nil {
			return err
		}

		if err := envx.ExpandInplace(filepath.Join(b.builddir, root, "Contents", "Info.plist"), os.Getenv); err != nil {
			return err
		}

		runtime := b.runtime.
			Environ("DMG_VOLUME_NAME", fmt.Sprintf("%s.%s", b.name, runtime.GOARCH)).
			Environ("DMG_OUTPUT", stringsx.DefaultIfBlank(b.outputpath, fmt.Sprintf("%s.%s.dmg", b.name, runtime.GOARCH)))
		return shell.Run(
			ctx,
			runtime.Newf("mkisofs -V ${DMG_VOLUME_NAME} -D -R -apple -no-pad -o ${DMG_OUTPUT} %s", filepath.Join(b.builddir, root)),
		)
	}
}
