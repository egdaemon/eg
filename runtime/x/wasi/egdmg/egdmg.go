package egdmg

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
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

// New create a new dmg specification using the pattern and options.
func New(pattern string, options ...Option) Specification {
	return langx.Clone(Specification{
		name:       strings.TrimSuffix(eggit.PatternClean(pattern), "."),
		runtime:    shell.Runtime(),
		builddir:   egenv.EphemeralDirectory(),
		outputpath: egenv.WorkspaceDirectory(),
		outputname: Name(pattern),
	}, options...)
}

type Specification struct {
	name       string
	outputpath string
	outputname string
	builddir   string
	runtime    shell.Command
}

func Build(b Specification, archive string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		log.Println("DMG BUILD INITIATED")
		defer log.Println("DMG BUILD COMPLETED")

		root := fmt.Sprintf("%s.app", b.name)

		sruntime := b.runtime
		return shell.Run(
			ctx,
			sruntime.Newf("rsync -avL %s/ %s/", archive, filepath.Join(b.builddir, root)),
			sruntime.Newf("ln -fs /Applications %s", filepath.Join(b.builddir, "Applications")),
			sruntime.Newf(
				"mkisofs -D -R -apple -no-pad -V %s -o %s %s",
				root,
				filepath.Join(b.outputpath, b.outputname),
				filepath.Join(b.builddir, root),
			),
		)
	}
}

func root(paths ...string) string {
	return egenv.WorkspaceDirectory(filepath.Join(paths...))
}

// Path from the given pattern
func Path(pattern string) string {
	return root(Name(pattern))
}

// replaces the substitution values within the pattern, resulting in the final resulting archive file's name.
func Name(pattern string) string {
	return fmt.Sprintf("%s.dmg", eggit.StringReplace(pattern))
}
