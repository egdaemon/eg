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
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/stringsx"
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
		root := fmt.Sprintf("%s.app", b.name)
		defer log.Println("DERP DERP", filepath.Join(b.builddir, root))
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
			runtime.Newf("ln -fs /Applications %s", filepath.Join(b.builddir, "Applications")),
			runtime.Newf("mkisofs -V ${DMG_VOLUME_NAME} -D -R -apple -no-pad -o ${DMG_OUTPUT} %s", filepath.Join(b.builddir, root)),
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
