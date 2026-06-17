package events_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/interp/events"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("duckdb", "")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	require.NoError(t, events.PrepareDB(ctx, db))

	return db
}

func TestWorstCoverage(t *testing.T) {
	db := newTestDB(t)
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	require.NoError(t, events.RecordMetric(
		ctx, db,
		events.NewCoverage(&events.Coverage{Path: "a.go", FnName: "Foo", Hits: 5, Statements: 80, Branches: 80}),
		events.NewCoverage(&events.Coverage{Path: "a.go", FnName: "Bar", Hits: 0, Statements: 80, Branches: 80}),
		events.NewCoverage(&events.Coverage{Path: "b.go", FnName: "Baz", Hits: 2, Statements: 60, Branches: 60}),
		events.NewCoverage(&events.Coverage{Path: "b.go", Statements: 60, Branches: 60}),
	))

	worst, err := events.WorstCoverage(ctx, db, 2)
	require.NoError(t, err)
	require.Len(t, worst, 2)
	require.Equal(t, "Bar", worst[0].FnName)
	require.Equal(t, int64(0), worst[0].Hits)
	require.Equal(t, "Baz", worst[1].FnName)
	require.Equal(t, int64(2), worst[1].Hits)
}

func TestSampleCoverage(t *testing.T) {
	db := newTestDB(t)
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	require.NoError(t, events.RecordMetric(
		ctx, db,
		events.NewCoverage(&events.Coverage{Path: "a.go", FnName: "Foo", Hits: 5}),
		events.NewCoverage(&events.Coverage{Path: "a.go", FnName: "Bar", Hits: 0}),
		events.NewCoverage(&events.Coverage{Path: "b.go", FnName: "Baz", Hits: 2}),
		events.NewCoverage(&events.Coverage{Path: "b.go"}),
	))

	sample, err := events.SampleCoverage(ctx, db, 2)
	require.NoError(t, err)
	require.Len(t, sample, 2)
	for _, c := range sample {
		require.NotEmpty(t, c.FnName)
	}
}
