package egdmg

import (
	"context"
	"fmt"
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

// experimental mkisofs
func OptionMkisofs(d *Specification) {
	d.cmd = "mkisofs -D -R -apple -no-pad -V %dmg.volume.name% -o %dmg.volume.output% %dmg.src.directory%"
}

// OptionDmgCmd set the command to build the dmg.
// the string replacements must be in the following order:
func OptionDmgCmd(s string) Option {
	return func(d *Specification) {
		d.cmd = s
	}
}

// New create a new dmg specification using the pattern and options.
func New(pattern string, options ...Option) Specification {
	return langx.Clone(Specification{
		name:       strings.TrimSuffix(eggit.PatternClean(pattern), "."),
		cmd:        "hdiutil create -fs HFS+ -volname \"%dmg.volume.name%\" -srcfolder %dmg.src.directory% %dmg.volume.output%",
		runtime:    shell.Runtime(),
		builddir:   egenv.EphemeralDirectory(),
		outputpath: egenv.WorkspaceDirectory(),
		outputname: Name(pattern),
	}, options...)
}

type Specification struct {
	name       string
	cmd        string
	outputpath string
	outputname string
	builddir   string
	runtime    shell.Command
}

func Build(b Specification, archive string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		root := fmt.Sprintf("%s.app", b.name)
		cmd := strings.ReplaceAll(b.cmd, "%dmg.volume.name%", root)
		cmd = strings.ReplaceAll(cmd, "%dmg.volume.output%", filepath.Join(b.outputpath, b.outputname))
		cmd = strings.ReplaceAll(cmd, "%dmg.src.directory%", filepath.Join(b.builddir, root))
		sruntime := b.runtime
		return shell.Run(
			ctx,
			sruntime.Newf("cp -R %s/ %s/", archive, filepath.Join(b.builddir, root)),
			sruntime.Newf("ln -fs /Applications %s", filepath.Join(b.builddir, "Applications")),
			sruntime.New(cmd),
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
