package egdmg

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

type Option func(*Specification)

func OptionIconPath(s string) Option {
	return func(d *Specification) {
		d.icon = s
	}
}

func New(name string, options ...Option) Specification {
	return langx.Clone(Specification{
		name: name,
	}, options...)
}

type Specification struct {
	icon string
	name string
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

		return nil
	}
}
