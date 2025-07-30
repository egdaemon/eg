package egdmg

import (
	"context"
	"fmt"
	"io/fs"
	"os"
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

func New(name string, options ...Option) Specification {
	return langx.Clone(Specification{
		name:    name,
		runtime: shell.Runtime(),
	}, options...)
}

type Specification struct {
	name       string
	outputpath string
	runtime    shell.Command
}

func Build(b Specification, archive fs.FS) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		if err := os.Symlink("/Applications", egenv.EphemeralDirectory("Applications")); err != nil {
			return err
		}

		root := fmt.Sprintf("%s.app", b.name)
		if err := egfs.CloneFS(ctx, egenv.EphemeralDirectory(root), ".", archive); err != nil {
			return err
		}

		if err := envx.ExpandInplace(egenv.EphemeralDirectory(root, "Contents", "Info.plist"), os.Getenv); err != nil {
			return err
		}

		runtime := b.runtime.
			Environ("DMG_VOLUME_NAME", fmt.Sprintf("%s.%s", b.name, runtime.GOARCH)).
			Environ("DMG_OUTPUT", stringsx.DefaultIfBlank(b.outputpath, fmt.Sprintf("%s.%s.dmg", b.name, runtime.GOARCH)))
		return shell.Run(
			ctx,
			runtime.Newf("mkisofs -V ${DMG_VOLUME_NAME} -D -R -apple -no-pad -o ${DMG_OUTPUT} %s", egenv.EphemeralDirectory(root)),
		)
	}
}
