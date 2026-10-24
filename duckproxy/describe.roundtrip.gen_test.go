package duckproxy_test

import (
	"context"
	"testing"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/stretchr/testify/require"
)

func TestDescribeRoundtrip(t *testing.T) {
	socketPath, _ := startTestServer(t)
	db := connect(t, socketPath)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "CREATE TABLE t (id INTEGER, name VARCHAR)")
	require.NoError(t, err)

	sqlConn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer sqlConn.Close()

	t.Run("tuple_returning_statement", func(t *testing.T) {
		result, err := duckproxy.Describe(ctx, sqlConn, "SELECT id, name FROM t WHERE id = $1")
		require.NoError(t, err)
		require.True(t, result.Tuples)
		require.Equal(t, uint32(duckdb.STATEMENT_TYPE_SELECT), result.StatementType)
		require.Len(t, result.Params, 1)
		require.Len(t, result.Columns, 2)
		require.Equal(t, "id", result.Columns[0].Name)
		require.Equal(t, uint32(duckdb.TYPE_INTEGER), result.Columns[0].Type)
		require.Equal(t, "name", result.Columns[1].Name)
		require.Equal(t, uint32(duckdb.TYPE_VARCHAR), result.Columns[1].Type)
	})

	t.Run("non_tuple_returning_statement", func(t *testing.T) {
		result, err := duckproxy.Describe(ctx, sqlConn, "INSERT INTO t VALUES ($1, $2)")
		require.NoError(t, err)
		require.False(t, result.Tuples)
		require.Equal(t, uint32(duckdb.STATEMENT_TYPE_INSERT), result.StatementType)
		require.Len(t, result.Params, 2)
		require.Empty(t, result.Columns)
	})

	t.Run("does_not_execute", func(t *testing.T) {
		// Describing an INSERT must not actually insert anything.
		var before, after int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&before))

		_, err := duckproxy.Describe(ctx, sqlConn, "INSERT INTO t VALUES (99, 'should-not-be-inserted')")
		require.NoError(t, err)

		require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&after))
		require.Equal(t, before, after)
	})

	t.Run("invalid_sql_propagates_an_error", func(t *testing.T) {
		_, err := duckproxy.Describe(ctx, sqlConn, "SELECT FROM nowhere if")
		require.Error(t, err)
	})
}
