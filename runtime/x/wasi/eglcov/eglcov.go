// Package eglcov provides the functionality to report test coverage from lcov files within a directory.
package eglcov

import (
	"context"

	"github.com/egdaemon/eg/internal/coverage/lcov"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoverage"
)

// report coverage from lcov files within a directory.
func ReportCoverage(dir string) eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		batch := make([]*events.Coverage, 0, 128)
		for rep, err := range lcov.Coverage(ctx, dir) {
			if err != nil {
				return err
			}

			batch = append(batch, rep)

			if len(batch) == cap(batch) {
				if err := fficoverage.Report(ctx, batch...); err != nil {
					return err
				}
				batch = batch[:0]
			}
		}

		if err := fficoverage.Report(ctx, batch...); err != nil {
			return err
		}

		return nil
	})
}
