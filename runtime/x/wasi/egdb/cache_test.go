package egdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"
)

func TestSnapshot(t *testing.T) {
	ctx := context.Background()

	live, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "analytics.db"))
	require.NoError(t, err)
	defer live.Close()
	_, err = live.Exec(`CREATE TABLE "eg.metrics.coverage" (name VARCHAR); INSERT INTO "eg.metrics.coverage" VALUES ('a'), ('b')`)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), ".eg", "default", "latest.db")
	require.NoError(t, snapshot(ctx, live, path))

	count := func() int {
		cached, err := sql.Open("duckdb", path+"?access_mode=read_only")
		require.NoError(t, err)
		defer cached.Close()
		var n int
		require.NoError(t, cached.QueryRow(`SELECT count(*) FROM "eg.metrics.coverage"`).Scan(&n))
		return n
	}
	require.Equal(t, 2, count())

	// a later snapshot replaces the earlier one, and the live database is untouched.
	_, err = live.Exec(`INSERT INTO "eg.metrics.coverage" VALUES ('c')`)
	require.NoError(t, err)
	require.NoError(t, snapshot(ctx, live, path))
	require.Equal(t, 3, count())

	var n int
	require.NoError(t, live.QueryRow(`SELECT count(*) FROM "eg.metrics.coverage"`).Scan(&n))
	require.Equal(t, 3, n)
}
