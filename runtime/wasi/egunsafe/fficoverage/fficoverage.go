package fficoverage

import (
	"context"
	"database/sql"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

// Coverage is a coverage measurement for a file, or for a function within it when Fnname is set.
type Coverage = coverage.Report

func dial() *sql.DB {
	dsn := egunsafe.RuntimeDirectory(eg.SocketAnalytics)
	return sql.OpenDB(duckproxy.NewConnector(dsn, egunsafe.DialAnalyticsSocket))
}

func report(ctx context.Context, q sqlx.Queryer, batch ...*coverage.Report) error {
	rows := make([]coverage.Report, 0, len(batch))
	for _, r := range batch {
		rows = append(rows, *r)
	}

	return sqlx.Discard(sqlx.Scan(coverage.NewReportBatchInsert(ctx, q, rows...)))
}

func worst(ctx context.Context, q sqlx.Queryer, n int32, filters ...coverage.Filter) (results []coverage.Report, err error) {
	err = sqlx.ScanInto(coverage.WorstWhere(ctx, q, int(n), filters...), &results)
	return results, err
}

func sample(ctx context.Context, q sqlx.Queryer, n int32, filters ...coverage.Filter) (results []coverage.Report, err error) {
	err = sqlx.ScanInto(coverage.SampleWhere(ctx, q, int(n), filters...), &results)
	return results, err
}

// Report inserts batch into the analytics database. the time allowed is proportional to the size of the batch.
func Report(ctx context.Context, batch ...*Coverage) error {
	// reportTimeoutPerItem is the time allowed per item in a batch when reporting.
	const reportTimeoutPerItem = 30 * time.Millisecond
	delta := time.Duration(len(batch)+1) * reportTimeoutPerItem
	ctx, done := context.WithTimeout(ctx, delta)
	defer done()

	db := dial()
	defer db.Close()

	return errorsx.Wrap(report(ctx, db, batch...), "unable to report coverage")
}

// Worst returns the n functions with the lowest hit counts, of those matching the filters.
func Worst(ctx context.Context, n int32, filters ...coverage.Filter) ([]coverage.Report, error) {
	db := dial()
	defer db.Close()

	results, err := worst(ctx, db, n, filters...)
	return results, errorsx.Wrap(err, "unable to query worst coverage")
}

// Sample returns a random sample of n functions, of those matching the filters.
func Sample(ctx context.Context, n int32, filters ...coverage.Filter) ([]coverage.Report, error) {
	db := dial()
	defer db.Close()

	results, err := sample(ctx, db, n, filters...)
	return results, errorsx.Wrap(err, "unable to query sample coverage")
}
