package fficoverage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/interp/events"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("duckdb", "")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	require.NoError(t, events.PrepareDB(ctx, db))

	return db
}

func TestReportWorstSample(t *testing.T) {
	db := newTestDB(t)
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	require.NoError(t, report(ctx, db,
		&coverage.Report{Path: "a.go", Fnname: "Foo", Hits: 5, Statements: 80, Branches: 80},
		&coverage.Report{Path: "a.go", Fnname: "Bar", Hits: 0, Statements: 80, Branches: 80},
		&coverage.Report{Path: "b.go", Fnname: "Baz", Hits: 2, Statements: 60, Branches: 60},
	))

	worstResults, err := worst(ctx, db, 2)
	require.NoError(t, err)
	require.Len(t, worstResults, 2)
	require.Equal(t, "Bar", worstResults[0].Fnname)
	require.Equal(t, int64(0), worstResults[0].Hits)
	require.Equal(t, "Baz", worstResults[1].Fnname)
	require.Equal(t, int64(2), worstResults[1].Hits)

	sampleResults, err := sample(ctx, db, 2)
	require.NoError(t, err)
	require.Len(t, sampleResults, 2)
	for _, c := range sampleResults {
		require.NotEmpty(t, c.Fnname)
	}
}

func TestWorstSampleFilters(t *testing.T) {
	db := newTestDB(t)
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	require.NoError(t, report(ctx, db,
		&coverage.Report{Path: "ui/app.ts", Fnname: "render", Hits: 0},
		&coverage.Report{Path: "tools/gen.py", Fnname: "generate", Hits: 0},
		&coverage.Report{Path: "cmd/eg/main.go", Fnname: "Main", Hits: 1},
		&coverage.Report{Path: "vendor/lib/lib.go", Fnname: "Vendored", Hits: 2},
		&coverage.Report{Path: "internal/a/a.go", Fnname: "A", Hits: 3},
		&coverage.Report{Path: "internal/b/b.go", Fnname: "B", Hits: 4},
		&coverage.Report{Path: "internal/c/c.go", Fnname: "", Hits: 0},
	))

	names := func(results []coverage.Report) (dst []string) {
		for _, r := range results {
			dst = append(dst, r.Fnname)
		}
		return dst
	}

	t.Run("language is applied before the limit", func(t *testing.T) {
		results, err := worst(ctx, db, 1, coverage.QueryLanguage("go"))
		require.NoError(t, err)
		require.Equal(t, []string{"Main"}, names(results))
	})

	t.Run("language is not case sensitive and accepts several", func(t *testing.T) {
		results, err := worst(ctx, db, 3, coverage.QueryLanguage("TypeScript", "Python"))
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"render", "generate"}, names(results))
	})

	t.Run("an unknown language is an error", func(t *testing.T) {
		_, err := worst(ctx, db, 1, coverage.QueryLanguage("klingon"))
		require.ErrorContains(t, err, `unknown language "klingon"`)
	})

	t.Run("paths include matching files, separators included", func(t *testing.T) {
		results, err := worst(ctx, db, 10, coverage.QueryPaths("internal/*"))
		require.NoError(t, err)
		require.Equal(t, []string{"A", "B"}, names(results))

		results, err = worst(ctx, db, 10, coverage.QueryPaths("cmd/*", "vendor/*"))
		require.NoError(t, err)
		require.Equal(t, []string{"Main", "Vendored"}, names(results))
	})

	t.Run("excluded paths are removed", func(t *testing.T) {
		results, err := worst(ctx, db, 10, coverage.QueryLanguage("go"), coverage.QueryExcludePaths("vendor/*", "internal/b/*"))
		require.NoError(t, err)
		require.Equal(t, []string{"Main", "A"}, names(results))
	})

	t.Run("filters combine", func(t *testing.T) {
		results, err := worst(ctx, db, 10, coverage.QueryLanguage("go"), coverage.QueryPaths("internal/*"), coverage.QueryExcludePaths("*/b/*"))
		require.NoError(t, err)
		require.Equal(t, []string{"A"}, names(results))
	})

	t.Run("without filters every function is selected", func(t *testing.T) {
		results, err := worst(ctx, db, 10)
		require.NoError(t, err)
		require.Len(t, results, 6)
	})

	t.Run("sample is filtered", func(t *testing.T) {
		for range 5 {
			results, err := sample(ctx, db, 2, coverage.QueryLanguage("go"), coverage.QueryExcludePaths("vendor/*"))
			require.NoError(t, err)
			require.Len(t, results, 2)
			for _, r := range results {
				require.Contains(t, []string{"Main", "A", "B"}, r.Fnname)
			}
		}
	})
}
