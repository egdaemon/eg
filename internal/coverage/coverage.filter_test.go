package coverage_test

import (
	"database/sql"
	"testing"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/sqlx"
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
		hits BIGINT NOT NULL DEFAULT 0
	)`)
	sqltestx.Exec(t, db, `INSERT INTO 'eg.metrics.coverage' (path, statements, branches, fnname, hits) VALUES
		('ui/app.ts', 0, 1, 'render', 0),
		('ui/app.tsx', 10, 1, 'component', 1),
		('tools/gen.py', 20, 1, 'generate', 0),
		('cmd/eg/main.go', 40, 1, 'Main', 2),
		('vendor/lib/lib.go', 60, 1, 'Vendored', 3),
		('internal/a/a.go', 80, 1, 'A', 4),
		('internal/b/b.go', 100, 1, 'B', 5),
		('internal/c/c.go', 1, 1, '', 0)`)

	worst := func(t *testing.T, q sqlx.Queryer, n int, filters ...coverage.Filter) ([]string, error) {
		var results []coverage.Report
		if err := sqlx.ScanInto(coverage.ReportSearch(t.Context(), q, coverage.WorstSearchBuilder(filters...).Limit(uint64(n))), &results); err != nil {
			return nil, err
		}

		names := make([]string, 0, len(results))
		for _, r := range results {
			names = append(names, r.Fnname)
		}
		return names, nil
	}

	t.Run("without filters the lowest statement coverage is first", func(t *testing.T) {
		names, err := worst(t, db, 4)
		require.NoError(t, err)
		require.Equal(t, []string{"render", "component", "generate", "Main"}, names)
	})

	t.Run("functions without a name are never selected", func(t *testing.T) {
		names, err := worst(t, db, 100)
		require.NoError(t, err)
		require.Len(t, names, 7)
		require.NotContains(t, names, "")
	})

	t.Run("the limit is of the functions that match", func(t *testing.T) {
		names, err := worst(t, db, 2, coverage.QueryLanguage("go"))
		require.NoError(t, err)
		require.Equal(t, []string{"Main", "Vendored"}, names)
	})

	t.Run("language matches every extension of the language and several languages", func(t *testing.T) {
		names, err := worst(t, db, 10, coverage.QueryLanguage("typescript"))
		require.NoError(t, err)
		require.Equal(t, []string{"render", "component"}, names)

		names, err = worst(t, db, 10, coverage.QueryLanguage("TypeScript", "python"))
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"render", "component", "generate"}, names)
	})

	t.Run("language without names does not restrict", func(t *testing.T) {
		names, err := worst(t, db, 100, coverage.QueryLanguage())
		require.NoError(t, err)
		require.Len(t, names, 7)
	})

	t.Run("an unknown language is an error naming the known languages", func(t *testing.T) {
		_, err := worst(t, db, 1, coverage.QueryLanguage("go", "klingon"))
		require.ErrorContains(t, err, `unknown language "klingon"`)
		require.ErrorContains(t, err, "go,")
	})

	t.Run("paths match the whole path, separators included", func(t *testing.T) {
		names, err := worst(t, db, 10, coverage.QueryPaths("internal/*"))
		require.NoError(t, err)
		require.Equal(t, []string{"A", "B"}, names)

		names, err = worst(t, db, 10, coverage.QueryPaths("internal"))
		require.NoError(t, err)
		require.Empty(t, names)

		names, err = worst(t, db, 10, coverage.QueryPaths("cmd/eg/mai?.go"))
		require.NoError(t, err)
		require.Equal(t, []string{"Main"}, names)
	})

	t.Run("excluded paths are removed", func(t *testing.T) {
		names, err := worst(t, db, 10, coverage.QueryLanguage("go"), coverage.QueryExcludePaths("vendor/*", "internal/b/*"))
		require.NoError(t, err)
		require.Equal(t, []string{"Main", "A"}, names)
	})

	t.Run("repeated patterns are harmless", func(t *testing.T) {
		names, err := worst(t, db, 10, coverage.QueryPaths("internal/*", "internal/*"), coverage.QueryExcludePaths("*/b/*", "*/b/*"))
		require.NoError(t, err)
		require.Equal(t, []string{"A"}, names)
	})

	t.Run("filters combine", func(t *testing.T) {
		names, err := worst(t, db, 10, coverage.QueryLanguage("go"), coverage.QueryPaths("internal/*", "cmd/*"), coverage.QueryExcludePaths("*/b/*"))
		require.NoError(t, err)
		require.Equal(t, []string{"Main", "A"}, names)
	})

	t.Run("coverage range includes both bounds", func(t *testing.T) {
		names, err := worst(t, db, 10, coverage.QueryCoverage(10, 60))
		require.NoError(t, err)
		require.Equal(t, []string{"component", "generate", "Main", "Vendored"}, names)
	})

	t.Run("coverage range is limited after it is applied", func(t *testing.T) {
		names, err := worst(t, db, 2, coverage.QueryCoverage(20, 100))
		require.NoError(t, err)
		require.Equal(t, []string{"generate", "Main"}, names)
	})

	t.Run("nothing matching is not an error", func(t *testing.T) {
		names, err := worst(t, db, 10, coverage.QueryLanguage("rust"))
		require.NoError(t, err)
		require.Empty(t, names)
	})

	t.Run("sample is filtered and limited", func(t *testing.T) {
		for range 5 {
			var results []coverage.Report
			require.NoError(t, sqlx.ScanInto(coverage.ReportSearch(t.Context(), db, coverage.SampleSearchBuilder(coverage.QueryLanguage("go"), coverage.QueryExcludePaths("vendor/*")).Limit(2)), &results))
			require.Len(t, results, 2)
			for _, r := range results {
				require.Contains(t, []string{"Main", "A", "B"}, r.Fnname)
			}
		}
	})

	t.Run("a failing query is reported", func(t *testing.T) {
		closed, err := sql.Open("duckdb", "")
		require.NoError(t, err)
		require.NoError(t, closed.Close())

		_, err = worst(t, closed, 1)
		require.Error(t, err)
	})
}
