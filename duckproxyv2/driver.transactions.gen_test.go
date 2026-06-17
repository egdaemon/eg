package duckproxyv2_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDriverTransactions(t *testing.T) {
	socketPath, _ := startTestServer(t)
	db := connect(t, socketPath)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "CREATE TABLE t (id INTEGER)")
	require.NoError(t, err)

	t.Run("commit_persists", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		_, err = tx.ExecContext(ctx, "INSERT INTO t VALUES (1)")
		require.NoError(t, err)
		require.NoError(t, tx.Commit())

		var n int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&n))
		require.Equal(t, 1, n)
	})

	t.Run("rollback_discards", func(t *testing.T) {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		_, err = tx.ExecContext(ctx, "INSERT INTO t VALUES (2)")
		require.NoError(t, err)
		require.NoError(t, tx.Rollback())

		var n int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&n))
		require.Equal(t, 1, n, "rolled back insert should not be visible")
	})
}
