package duckproxyv2_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDriverQueryContext(t *testing.T) {
	socketPath, _ := startTestServer(t)
	db := connect(t, socketPath)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "CREATE TABLE t (id INTEGER, name VARCHAR)")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO t VALUES ($1, $2)", 1, "alice")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO t VALUES ($1, $2)", 2, "bob")
	require.NoError(t, err)

	t.Run("select_with_no_rows", func(t *testing.T) {
		rows, err := db.QueryContext(ctx, "SELECT id, name FROM t WHERE id = $1", 999)
		require.NoError(t, err)
		defer rows.Close()

		require.False(t, rows.Next())
		require.NoError(t, rows.Err())
	})

	t.Run("select_with_multiple_rows", func(t *testing.T) {
		rows, err := db.QueryContext(ctx, "SELECT id, name FROM t ORDER BY id")
		require.NoError(t, err)
		defer rows.Close()

		var got []string
		for rows.Next() {
			var id int
			var name string
			require.NoError(t, rows.Scan(&id, &name))
			got = append(got, name)
		}
		require.NoError(t, rows.Err())
		require.Equal(t, []string{"alice", "bob"}, got)
	})

	t.Run("parameterized_where", func(t *testing.T) {
		var name string
		err := db.QueryRowContext(ctx, "SELECT name FROM t WHERE id = $1", 2).Scan(&name)
		require.NoError(t, err)
		require.Equal(t, "bob", name)
	})
}
