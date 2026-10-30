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

// TestLegacyCoverageDispatch confirms the original Message_Coverage path
// (still used by already-released SDK versions) keeps inserting correctly,
// leaving the newer fnname/hits columns at their defaults.
func TestLegacyCoverageDispatch(t *testing.T) {
	db := newTestDB(t)
	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	require.NoError(t, events.RecordMetric(
		ctx, db,
		events.NewCoverage(&events.Coverage{Path: "a.go", Statements: 80, Branches: 60}),
	))

	var (
		path                 string
		statements, branches float32
		fnname               string
		hits                 int64
	)
	row := db.QueryRowContext(ctx, "SELECT path, statements, branches, fnname, hits FROM 'eg.metrics.coverage' WHERE path = ?", "a.go")
	require.NoError(t, row.Scan(&path, &statements, &branches, &fnname, &hits))
	require.Equal(t, "a.go", path)
	require.Equal(t, float32(80), statements)
	require.Equal(t, float32(60), branches)
	require.Equal(t, "", fnname)
	require.Equal(t, int64(0), hits)
}

// TestPrepareDBPreservesLegacyCoverage confirms databases created before migrations were tracked, and so
// before the generated column existed, keep their coverage rows.
func TestPrepareDBPreservesLegacyCoverage(t *testing.T) {
	db, err := sql.Open("duckdb", "")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	ctx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()

	_, err = db.ExecContext(ctx, `CREATE TABLE 'eg.metrics.coverage' (id UUID PRIMARY KEY DEFAULT uuidv7(), path TEXT NOT NULL, statements FLOAT4 NOT NULL, branches FLOAT4 NOT NULL, fnname TEXT NOT NULL DEFAULT '', hits BIGINT NOT NULL DEFAULT 0)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO 'eg.metrics.coverage' (path, statements, branches, fnname, hits) VALUES ('a.go', 80, 60, 'Fn', 3)`)
	require.NoError(t, err)

	require.NoError(t, events.PrepareDB(ctx, db))

	var (
		path      string
		hits      int64
		generated bool
	)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT path, hits, generated FROM 'eg.metrics.coverage'`).Scan(&path, &hits, &generated))
	require.Equal(t, "a.go", path)
	require.EqualValues(t, 3, hits)
	require.False(t, generated)

	var leftover int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM duckdb_tables() WHERE table_name = 'eg.metrics.coverage_old'`).Scan(&leftover))
	require.Zero(t, leftover)
}
