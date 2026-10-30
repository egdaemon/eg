package egcoverage_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/x/wasi/egcoverage"
	"github.com/stretchr/testify/require"
)

// TestReportedCoverageIsQueryable confirms what an operation reports can be selected again, worst first and sampled,
// with the filters applied before the limit.
func TestReportedCoverageIsQueryable(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	require.NoError(t, events.PrepareDB(ctx, db))

	sink := func(ctx context.Context, batch ...*coverage.Report) error {
		rows := make([]coverage.Report, 0, len(batch))
		for _, r := range batch {
			rows = append(rows, *r)
		}

		return sqlx.Discard(sqlx.Scan(coverage.NewReportBatchInsert(ctx, db, rows...)))
	}

	reports := coverage.New(func(context.Context) coverage.Seq {
		return func(yield func(*coverage.Report, error) bool) {
			for _, r := range []*coverage.Report{
				{Path: "a.go", Fnname: "Foo", Hits: 5, Statements: 80, Branches: 80},
				{Path: "a.go", Fnname: "Bar", Hits: 0, Statements: 80, Branches: 80},
				{Path: "b.go", Fnname: "Baz", Hits: 2, Statements: 60, Branches: 60},
				{Path: "vendor/lib/lib.go", Fnname: "Vendored", Hits: 1, Statements: 90, Branches: 90},
				{Path: "ui/app.ts", Fnname: "render", Hits: 1, Statements: 95, Branches: 95},
			} {
				if !yield(r, nil) {
					return
				}
			}
		}
	})

	require.NoError(t, egcoverage.Into(sink, reports...)(ctx, nil))

	names := func(results []coverage.Report) (dst []string) {
		for _, r := range results {
			dst = append(dst, r.Fnname)
		}
		return dst
	}

	t.Run("worst is the lowest statement coverage first, then the fewest hits", func(t *testing.T) {
		var results []coverage.Report
		require.NoError(t, sqlx.ScanInto(coverage.ReportSearch(ctx, db, coverage.WorstSearchBuilder().Limit(3)), &results))
		require.Equal(t, []string{"Baz", "Bar", "Foo"}, names(results))
		require.Equal(t, int64(2), results[0].Hits)
		require.Equal(t, int64(0), results[1].Hits)
	})

	t.Run("filters are applied before the limit", func(t *testing.T) {
		var results []coverage.Report
		require.NoError(t, sqlx.ScanInto(coverage.ReportSearch(ctx, db, coverage.WorstSearchBuilder(coverage.QueryLanguage("go"), coverage.QueryExcludePaths("vendor/*")).Limit(1)), &results))
		require.Equal(t, []string{"Baz"}, names(results))
	})

	t.Run("sample is filtered and limited", func(t *testing.T) {
		for range 5 {
			var results []coverage.Report
			require.NoError(t, sqlx.ScanInto(coverage.ReportSearch(ctx, db, coverage.SampleSearchBuilder(coverage.QueryLanguage("go"), coverage.QueryExcludePaths("vendor/*")).Limit(2)), &results))
			require.Len(t, results, 2)
			for _, r := range results {
				require.Contains(t, []string{"Foo", "Bar", "Baz"}, r.Fnname)
			}
		}
	})
}
