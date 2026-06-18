package duckpgwire_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSimpleHandleQuery(t *testing.T) {
	dir, _ := startTestServer(t)
	conn := connect(t, dir, true)
	ctx := context.Background()

	_, err := conn.Exec(ctx, "CREATE TABLE t (id INTEGER, name VARCHAR)")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "INSERT INTO t VALUES (1, 'a')")
	require.NoError(t, err)

	rows, err := conn.Query(ctx, "SELECT id, name FROM t")
	require.NoError(t, err)
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int32
		var name string
		require.NoError(t, rows.Scan(&id, &name))
		require.Equal(t, int32(1), id)
		require.Equal(t, "a", name)
		count++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, 1, count)
}
