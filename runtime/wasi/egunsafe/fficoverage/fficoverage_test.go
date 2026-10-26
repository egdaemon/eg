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
