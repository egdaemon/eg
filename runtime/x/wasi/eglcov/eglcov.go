// Package eglcov provides the coverage of lcov files, recorded with egcoverage.Report.
package eglcov

import (
	"context"
	"io"
	"path/filepath"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/coverage/lcov"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

// Coverage is the coverage of the lcov files within a directory, with paths relative to the workspace. lcov paths are
// relative to the project that wrote them, which is taken to be the parent of dir, e.g.) the project of
// console/coverage is console. use Root to anchor them elsewhere, e.g.) Coverage(dir).Root(egenv.DirectoryWorking("other")).
func Coverage(dir string) coverage.Options {
	return coverage.New(func(ctx context.Context) coverage.Seq {
		return lcov.Coverage(ctx, dir)
	}).Root(filepath.Dir(dir)).Transforms(coverage.TransformPath(egenv.PathRelativeWorkspace))
}

// CoverageFrom is the coverage of an lcov document, see Coverage.
func CoverageFrom(r io.Reader) coverage.Options {
	return coverage.New(func(ctx context.Context) coverage.Seq {
		return lcov.Parse(ctx, r)
	}).Transforms(coverage.TransformPath(egenv.PathRelativeWorkspace))
}
