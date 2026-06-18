package duckproxy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDriverExecContext(t *testing.T) {
	socketPath, _ := startTestServer(t)
	db := connect(t, socketPath)
	ctx := context.Background()

	t.Run("create_table", func(t *testing.T) {
		_, err := db.ExecContext(ctx, "CREATE TABLE t (id INTEGER, name VARCHAR)")
		require.NoError(t, err)
	})

	t.Run("insert_reports_rows_affected", func(t *testing.T) {
		res, err := db.ExecContext(ctx, "INSERT INTO t VALUES ($1, $2)", 1, "alice")
		require.NoError(t, err)

		n, err := res.RowsAffected()
		require.NoError(t, err)
		require.EqualValues(t, 1, n)
	})

	t.Run("failing_statement_propagates_an_error", func(t *testing.T) {
		_, err := db.ExecContext(ctx, "INSERT INTO does_not_exist VALUES (1)")
		require.Error(t, err)
	})
}
