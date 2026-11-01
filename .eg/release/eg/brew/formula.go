package main

import (
	"context"
	_ "embed"
	"text/template"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
	"github.com/egdaemon/eg/runtime/x/wasi/eggithub"
	"github.com/egdaemon/eg/runtime/x/wasi/egtarball"
	"github.com/egdaemon/eg/runtime/x/wasi/egtemplate"
)

//go:embed eg.rb.tmpl
var formulasrc string

var formula = template.Must(template.New("eg.rb").Parse(formulasrc))

// formulapath is where the rendered formula is written.
func formulapath() string {
	return egenv.WorkspaceDirectory("eg.rb")
}

// brew renders the homebrew formula for the archive pattern into formulapath.
// the template values are computed when the operation runs, so it must run after the archive has been packed.
func brew(pattern string) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		return egfs.Install(formulapath(), egtemplate.Reader(formula, struct {
			URL     string
			SHA256  string
			Version string
		}{
			URL:     eggithub.DownloadURL(pattern),
			SHA256:  egtarball.SHA256(pattern),
			Version: eggithub.PatternVersion(),
		}))(ctx, o)
	}
}
