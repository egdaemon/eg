package egdmg

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

type Option func(*Specification)

func OptionRuntime(s shell.Command) Option {
	return func(d *Specification) {
		d.runtime = s
	}
}

// directory to construct process the clone and then construct the file tree in.
// defaults to egenv.EphemeralDirectory
func OptionBuildDir(s string) Option {
	return func(d *Specification) {
		d.builddir = s
	}
}

// directory to place dmg, defaults to egenv.WorkloadDirectory
func OptionOutputDir(s string) Option {
	return func(d *Specification) {
		d.builddir = s
	}
}

// filename to give to the resulting dmg file.
// defaults to {name}.dmg
func OptionOutputName(s string) Option {
	return func(d *Specification) {
		d.builddir = s
	}
}

func New(name string, options ...Option) Specification {
	return langx.Clone(Specification{
		name:       name,
		runtime:    shell.Runtime(),
		builddir:   egenv.EphemeralDirectory(),
		outputpath: egenv.WorkspaceDirectory(),
		outputname: fmt.Sprintf("%s.dmg", name),
	}, options...)
}

type Specification struct {
	name       string
	outputpath string
	outputname string
	builddir   string
	runtime    shell.Command
}

func Build(b Specification, archive fs.FS) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		root := fmt.Sprintf("%s.app", b.name)
		if err := egfs.CloneFS(ctx, filepath.Join(b.builddir, root), ".", archive); err != nil {
			return err
		}

		log.Println("dmg source dir")
		fsx.PrintFS(os.DirFS(filepath.Join(b.builddir, root)))

		if err := envx.ExpandInplace(filepath.Join(b.builddir, root, "Contents", "Info.plist"), os.Getenv); err != nil {
			return err
		}

		sruntime := b.runtime
		return shell.Run(
			ctx,
			sruntime.Newf("ln -fs /Applications %s", filepath.Join(b.builddir, "Applications")),
			sruntime.Newf(
				"mkisofs -V %s -D -R -apple -no-pad -o %s %s",
				fmt.Sprintf("%s.%s", b.name, runtime.GOARCH),
				filepath.Join(b.outputpath, b.outputname),
				filepath.Join(b.builddir, root),
			),
		)
	}
}

func root(paths ...string) string {
	return egenv.CacheDirectory(".eg", filepath.Join(paths...))
}

// Path from the given pattern
func Path(pattern string) string {
	return root(Name(pattern))
}

// replaces the substitution values within the pattern, resulting in the final resulting archive file's name.
func Name(pattern string) string {
	c := eggit.EnvCommit()
	return fmt.Sprintf("%s.dmg", c.StringReplace(pattern))
}
