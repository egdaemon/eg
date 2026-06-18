package duckpgwire_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecTransactions(t *testing.T) {
	dir, _ := startTestServer(t)
	conn := connect(t, dir, false)
	ctx := context.Background()

	_, err := conn.Exec(ctx, "CREATE TABLE t3 (id INTEGER)")
	require.NoError(t, err)

	t.Run("commit_persists", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, "INSERT INTO t3 VALUES (1)")
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))

		var n int
		require.NoError(t, conn.QueryRow(ctx, "SELECT count(*) FROM t3").Scan(&n))
		require.Equal(t, 1, n, "expected the committed insert to be visible")
	})

	t.Run("rollback_discards", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, "INSERT INTO t3 VALUES (2)")
		require.NoError(t, err)
		require.NoError(t, tx.Rollback(ctx))

		var n int
		require.NoError(t, conn.QueryRow(ctx, "SELECT count(*) FROM t3").Scan(&n))
		require.Equal(t, 1, n, "rolled back insert should not be visible")
	})
}
