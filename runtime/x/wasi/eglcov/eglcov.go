// Package eglcov provides the coverage of lcov files, recorded with egcoverage.Report.
package eglcov

import (
	"context"
	"io"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/coverage/lcov"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

// Coverage is the coverage of the lcov files within a directory, with paths relative to the workspace. lcov paths are
// relative to their project, use Root to anchor them, e.g.) Coverage(dir).Root(egenv.WorkingDirectory("console")).
func Coverage(dir string) coverage.Options {
	return coverage.New(func(ctx context.Context) coverage.Seq {
		return lcov.Coverage(ctx, dir)
	}).Transforms(coverage.TransformPath(egenv.PathRelativeWorkspace))
}

// CoverageFrom is the coverage of an lcov document, see Coverage.
func CoverageFrom(r io.Reader) coverage.Options {
	return coverage.New(func(ctx context.Context) coverage.Seq {
		return lcov.Parse(ctx, r)
	}).Transforms(coverage.TransformPath(egenv.PathRelativeWorkspace))
}
