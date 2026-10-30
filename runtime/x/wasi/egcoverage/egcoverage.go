// Package egcoverage provides the operation that records test coverage into the analytics database.
package egcoverage

import (
	"context"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoverage"
)

// batchsize is the number of reports sent to the sink at once.
const batchsize = 128

// Report records the coverage the options produce into the analytics database. the options are built by the
// language specific packages, e.g.) eggolang.Coverage and eglcov.Coverage.
func Report(opts ...coverage.Option) eg.OpFn {
	return Into(fficoverage.Report, opts...)
}

// Into is Report with the sink injected, e.g.) for tests which can't dial the analytics socket.
func Into(report func(context.Context, ...*coverage.Report) error, opts ...coverage.Option) eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		batch := make([]*coverage.Report, 0, batchsize)
		for rep, err := range coverage.Options(opts).Seq(ctx) {
			if err != nil {
				return err
			}

			batch = append(batch, rep)

			if len(batch) == cap(batch) {
				if err := report(ctx, batch...); err != nil {
					return err
				}
				batch = batch[:0]
			}
		}

		return report(ctx, batch...)
	})
}
