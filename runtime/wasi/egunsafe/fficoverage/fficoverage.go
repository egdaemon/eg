package fficoverage

import (
	"context"
	"database/sql"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

func dial() *sql.DB {
	dsn := egunsafe.RuntimeDirectory(eg.SocketAnalytics)
	return sql.OpenDB(duckproxy.NewConnector(dsn, egunsafe.DialAnalyticsSocket))
}

func report(ctx context.Context, q sqlx.Queryer, batch ...*coverage.Report) error {
	rows := make([]coverage.Report, 0, len(batch))
	for _, r := range batch {
		rows = append(rows, *r)
	}

	return sqlx.Discard(coverage.NewReportBatchInsert(ctx, q, rows...))
}

func worst(ctx context.Context, q sqlx.Queryer, n int32) (results []coverage.Report, err error) {
	err = sqlx.ScanInto(coverage.Worst(ctx, q, int(n)), &results)
	return results, err
}

func sample(ctx context.Context, q sqlx.Queryer, n int32) (results []coverage.Report, err error) {
	err = sqlx.ScanInto(coverage.Sample(ctx, q, int(n)), &results)
	return results, err
}

// Report inserts batch into the analytics database.
func Report(ctx context.Context, batch ...*coverage.Report) error {
	db := dial()
	defer db.Close()

	return errorsx.Wrap(report(ctx, db, batch...), "unable to report coverage")
}

// Worst returns the n functions with the lowest hit counts.
func Worst(ctx context.Context, n int32) ([]coverage.Report, error) {
	db := dial()
	defer db.Close()

	results, err := worst(ctx, db, n)
	return results, errorsx.Wrap(err, "unable to query worst coverage")
}

// Sample returns a random sample of n functions.
func Sample(ctx context.Context, n int32) ([]coverage.Report, error) {
	db := dial()
	defer db.Close()

	results, err := sample(ctx, db, n)
	return results, errorsx.Wrap(err, "unable to query sample coverage")
}
