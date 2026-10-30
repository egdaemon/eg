package egautogen_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/runtime/x/wasi/egautogen"
	"github.com/stretchr/testify/require"
)

func TestFilters(t *testing.T) {
	db := sqltestx.DuckDB(t)
	sqltestx.Exec(t, db, `CREATE TABLE 'eg.metrics.coverage' (
		id UUID PRIMARY KEY DEFAULT uuidv7(),
		path TEXT NOT NULL,
		statements FLOAT4 NOT NULL,
		branches FLOAT4 NOT NULL,
		fnname TEXT NOT NULL DEFAULT '',
		hits BIGINT NOT NULL DEFAULT 0,
		generated BOOLEAN NOT NULL DEFAULT false
	)`)
	sqltestx.Exec(t, db, `INSERT INTO 'eg.metrics.coverage' (path, statements, branches, fnname, hits, generated) VALUES
		('a/low.go', 10, 1, 'Low', 0, false),
		('a/mid.go', 50, 1, 'Mid', 1, false),
		('a/high.go', 90, 1, 'High', 2, false),
		('a/low.pb.go', 20, 1, 'LowGenerated', 0, true),
		('ui/app.ts', 30, 1, 'Render', 0, false)`)

	names := func(t *testing.T, filters ...coverage.Filter) []string {
		var results []coverage.Report
		require.NoError(t, sqlx.ScanInto(coverage.ReportSearch(t.Context(), db, egautogen.Worst(filters...)), &results))

		found := make([]string, 0, len(results))
		for _, r := range results {
			found = append(found, r.Fnname)
		}
		return found
	}

	t.Run("a chain is the intersection of its filters", func(t *testing.T) {
		require.Equal(t, []string{"Low", "Mid"}, names(t, egautogen.Language("go").Coverage(0, 80).SkipGenerated()))
	})

	t.Run("without skipping, generated functions are kept", func(t *testing.T) {
		require.Equal(t, []string{"Low", "LowGenerated", "Mid"}, names(t, egautogen.Language("go").Coverage(0, 80)))
	})

	t.Run("a chain equals the filters passed separately", func(t *testing.T) {
		require.Equal(t,
			names(t, egautogen.Language("go"), egautogen.Coverage(0, 80), egautogen.SkipGenerated()),
			names(t, egautogen.Language("go").Coverage(0, 80).SkipGenerated()),
		)
	})

	t.Run("a chain can start anywhere and mix paths", func(t *testing.T) {
		require.Equal(t, []string{"Low"}, names(t, egautogen.SkipGenerated().Paths("a/*").ExcludePaths("*/mid.go", "*/high.go")))
	})

	t.Run("language without names does not restrict", func(t *testing.T) {
		require.Equal(t, []string{"Low", "LowGenerated", "Render", "Mid", "High"}, names(t, egautogen.Language()))
	})

	t.Run("chaining does not modify the receiver", func(t *testing.T) {
		base := egautogen.Language("go")
		_ = base.Coverage(90, 100)
		_ = base.SkipGenerated()

		require.Equal(t, []string{"Low", "LowGenerated", "Mid", "High"}, names(t, base))
	})

	t.Run("an unknown language is an error", func(t *testing.T) {
		var results []coverage.Report
		err := sqlx.ScanInto(coverage.ReportSearch(t.Context(), db, egautogen.Worst(egautogen.Language("go").Language("klingon"))), &results)
		require.ErrorContains(t, err, "unknown language")
	})
}
