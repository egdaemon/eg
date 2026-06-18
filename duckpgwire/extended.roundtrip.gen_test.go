package duckpgwire_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtendedRoundtrip(t *testing.T) {
	dir, _ := startTestServer(t)
	conn := connect(t, dir, false)
	ctx := context.Background()

	_, err := conn.Exec(ctx, "CREATE TABLE t2 (id INTEGER, name VARCHAR)")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "INSERT INTO t2 VALUES ($1, $2)", 1, "alice")
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "INSERT INTO t2 VALUES ($1, $2)", 2, "bob")
	require.NoError(t, err)

	var name string
	require.NoError(t, conn.QueryRow(ctx, "SELECT name FROM t2 WHERE id = $1", 2).Scan(&name))
	require.Equal(t, "bob", name)
}
