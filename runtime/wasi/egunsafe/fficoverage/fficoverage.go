package fficoverage

import (
	"context"
	"time"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sqlx"
)

// Coverage is a coverage measurement for a file, or for a function within it when Fnname is set.
type Coverage = coverage.Report

func report(ctx context.Context, q sqlx.Queryer, batch ...*coverage.Report) error {
	rows := make([]coverage.Report, 0, len(batch))
	for _, r := range batch {
		rows = append(rows, *r)
	}

	return sqlx.Discard(sqlx.Scan(coverage.NewReportBatchInsert(ctx, q, rows...)))
}

// Report inserts batch into the analytics database q, e.g.) the one egdb.Open returns, q belongs to the caller and is not
// closed. the time allowed is proportional to the size of the batch.
func Report(ctx context.Context, q sqlx.Queryer, batch ...*Coverage) error {
	// reportTimeoutPerItem is the time allowed per item in a batch when reporting.
	const reportTimeoutPerItem = 30 * time.Millisecond
	delta := time.Duration(len(batch)+1) * reportTimeoutPerItem
	ctx, done := context.WithTimeout(ctx, delta)
	defer done()

	return errorsx.Wrap(report(ctx, q, batch...), "unable to report coverage")
}
