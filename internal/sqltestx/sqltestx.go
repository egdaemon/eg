// Package sqltestx provides helpers for tests that need a throwaway duckdb database.
package sqltestx

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/stretchr/testify/require"
)

// Path returns a database file path within the test's temporary directory, for tests that
// need to close and reopen the same database.
func Path(t testing.TB) string {
	return filepath.Join(t.TempDir(), "test.duckdb")
}

// DuckDB opens an empty database in the test's temporary directory, closed when the test ends.
func DuckDB(t testing.TB) *sql.DB {
	db, err := sql.Open("duckdb", Path(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// Exec runs a statement, failing the test on error.
func Exec(t testing.TB, q sqlx.Queryer, query string, args ...any) {
	_, err := q.ExecContext(t.Context(), query, args...)
	require.NoError(t, err)
}

// Count returns the single integer produced by the query.
func Count(t testing.TB, q sqlx.Queryer, query string, args ...any) (count int) {
	return scan[int](t, q, query, args...)
}

// String returns the single string produced by the query.
func String(t testing.TB, q sqlx.Queryer, query string, args ...any) string {
	return scan[string](t, q, query, args...)
}

// Timestamp returns the single timestamp produced by the query.
func Timestamp(t testing.TB, q sqlx.Queryer, query string, args ...any) time.Time {
	return scan[time.Time](t, q, query, args...)
}

func scan[T any](t testing.TB, q sqlx.Queryer, query string, args ...any) (v T) {
	require.NoError(t, q.QueryRowContext(t.Context(), query, args...).Scan(&v))
	return v
}
