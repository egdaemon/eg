package duckpgwire_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/duckpgwire"
	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// the runner serves the postgres frontend on eg.SocketAnalyticsPG, the name libpq style clients derive from a
// directory, so a client given only the runtime directory must find it.
func TestServerSocketName(t *testing.T) {
	t.Run("client_finds_the_runner_socket_from_its_directory", func(t *testing.T) {
		dir := t.TempDir()
		proxyPath := filepath.Join(dir, eg.SocketAnalytics)
		pgPath := filepath.Join(dir, eg.SocketAnalyticsPG)

		db, err := sql.Open("duckdb", filepath.Join(dir, "test.duckdb"))
		require.NoError(t, err)
		t.Cleanup(func() { db.Close() })

		ctx, cancel := context.WithCancel(context.Background())
		proxyErr := make(chan error, 1)
		go func() { proxyErr <- duckproxyserver.ListenUnix(ctx, proxyPath, duckproxyserver.New(db)) }()
		waitForSocket(t, proxyPath)

		client, err := sql.Open("duckproxy", proxyPath)
		require.NoError(t, err)
		t.Cleanup(func() { client.Close() })

		pgErr := make(chan error, 1)
		go func() { pgErr <- duckpgwire.ListenUnix(ctx, pgPath, duckpgwire.New(client)) }()
		waitForSocket(t, pgPath)
		t.Cleanup(func() {
			cancel()
			<-pgErr
			<-proxyErr
		})

		cfg, err := pgx.ParseConfig("host=" + dir + " database=test sslmode=disable")
		require.NoError(t, err)

		conn, err := pgx.ConnectConfig(ctx, cfg)
		require.NoError(t, err)
		t.Cleanup(func() { conn.Close(context.Background()) })

		var n int
		require.NoError(t, conn.QueryRow(ctx, "SELECT 41 + 1").Scan(&n))
		require.Equal(t, 42, n)
	})
}
