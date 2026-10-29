package fficoverage

import (
	"context"
	"database/sql"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
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
	err = sqlx.ScanInto(coverage.ReportSearch(ctx, q, coverage.WorstSearchBuilder(filters...).Limit(uint64(max(n, 0)))), &results)
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

// Sample is the query selecting the functions matching the filters in a random order. it is not run, see
// coverage.ReportSearch, nor limited, apply Limit for a sample of n functions.
func Sample(filters ...coverage.Filter) squirrel.SelectBuilder {
	return coverage.SampleSearchBuilder(filters...)
}

// WorstFrom is the query Worst runs, the functions with the lowest statement coverage first, of those matching the filters.
// it is not run, see coverage.ReportSearch, so it can be run against another database, e.g.) a database file opened
// with egdb.OpenFile, or narrowed further, e.g.) limited.
func WorstFrom(filters ...coverage.Filter) squirrel.SelectBuilder {
	return coverage.WorstSearchBuilder(filters...)
}

// Worst iterates the functions with the lowest statement coverage first, of those matching the filters. the analytics
// database is dialed when iteration starts and closed when it ends, including when it stops early.
func Worst(filters ...coverage.Filter) iterx.Seq[coverage.Report] {
	return coverage.From(func(context.Context) (*sql.DB, error) { return dial(), nil }, WorstFrom(filters...))
}
