package egdb

import (
	"context"
	"database/sql"
	"net"
	"path/filepath"
	"sync"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/stretchr/testify/require"
)

func TestMigrate(t *testing.T) {
	t.Run("a fresh database is migrated and migrating again is a no-op", func(t *testing.T) {
		dir := t.TempDir()
		socket := filepath.Join(dir, "analytics.socket")

		host, err := sql.Open("duckdb", filepath.Join(dir, "analytics.db"))
		require.NoError(t, err)
		t.Cleanup(func() { host.Close() })

		sctx, stop := context.WithCancel(context.Background())
		served := make(chan error, 1)
		go func() { served <- duckproxyserver.ListenUnix(sctx, socket, duckproxyserver.New(host)) }()
		t.Cleanup(func() {
			stop()
			<-served
		})

		guest := sql.OpenDB(duckproxy.NewConnector(socket, (&net.Dialer{}).DialContext))
		t.Cleanup(func() { guest.Close() })
		require.Eventually(t, func() bool { return guest.PingContext(context.Background()) == nil }, 2e9, 1e7)

		require.NoError(t, migrate(t.Context(), guest))
		require.NoError(t, migrate(t.Context(), guest))

		var generated int
		require.NoError(t, host.QueryRow(`SELECT count(*) FROM duckdb_columns() WHERE table_name = 'eg.metrics.coverage' AND column_name = 'generated'`).Scan(&generated))
		require.Equal(t, 1, generated)
	})

	t.Run("a database created before migrations were tracked keeps its rows", func(t *testing.T) {
		dir := t.TempDir()
		socket := filepath.Join(dir, "analytics.socket")

		host, err := sql.Open("duckdb", filepath.Join(dir, "analytics.db"))
		require.NoError(t, err)
		t.Cleanup(func() { host.Close() })

		_, err = host.Exec(`CREATE TABLE 'eg.metrics.coverage' (id UUID PRIMARY KEY DEFAULT uuidv7(), path TEXT NOT NULL, statements FLOAT4 NOT NULL, branches FLOAT4 NOT NULL, fnname TEXT NOT NULL DEFAULT '', hits BIGINT NOT NULL DEFAULT 0)`)
		require.NoError(t, err)
		_, err = host.Exec(`INSERT INTO 'eg.metrics.coverage' (path, statements, branches, fnname, hits) VALUES ('a.go', 80, 60, 'Fn', 3)`)
		require.NoError(t, err)

		sctx, stop := context.WithCancel(context.Background())
		served := make(chan error, 1)
		go func() { served <- duckproxyserver.ListenUnix(sctx, socket, duckproxyserver.New(host)) }()
		t.Cleanup(func() {
			stop()
			<-served
		})

		guest := sql.OpenDB(duckproxy.NewConnector(socket, (&net.Dialer{}).DialContext))
		t.Cleanup(func() { guest.Close() })
		require.Eventually(t, func() bool { return guest.PingContext(context.Background()) == nil }, 2e9, 1e7)

		require.NoError(t, migrate(t.Context(), guest))

		var (
			path      string
			hits      int64
			generated bool
		)
		require.NoError(t, host.QueryRow(`SELECT path, hits, generated FROM 'eg.metrics.coverage'`).Scan(&path, &hits, &generated))
		require.Equal(t, "a.go", path)
		require.EqualValues(t, 3, hits)
		require.False(t, generated)
	})

	t.Run("concurrent guests all succeed", func(t *testing.T) {
		dir := t.TempDir()
		socket := filepath.Join(dir, "analytics.socket")

		host, err := sql.Open("duckdb", filepath.Join(dir, "analytics.db"))
		require.NoError(t, err)
		t.Cleanup(func() { host.Close() })

		sctx, stop := context.WithCancel(context.Background())
		served := make(chan error, 1)
		go func() { served <- duckproxyserver.ListenUnix(sctx, socket, duckproxyserver.New(host)) }()
		t.Cleanup(func() {
			stop()
			<-served
		})

		probe := sql.OpenDB(duckproxy.NewConnector(socket, (&net.Dialer{}).DialContext))
		t.Cleanup(func() { probe.Close() })
		require.Eventually(t, func() bool { return probe.PingContext(context.Background()) == nil }, 2e9, 1e7)

		var wg sync.WaitGroup
		errs := make([]error, 4)
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				guest := sql.OpenDB(duckproxy.NewConnector(socket, (&net.Dialer{}).DialContext))
				defer guest.Close()
				errs[i] = migrate(context.Background(), guest)
			}()
		}
		wg.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}
	})

	t.Run("disabled by the environment", func(t *testing.T) {
		t.Setenv(eg.EnvComputeGuestMigrationsEnabled, "false")

		db, err := sql.Open("duckdb", "")
		require.NoError(t, err)
		t.Cleanup(func() { db.Close() })

		require.NoError(t, migrate(t.Context(), db))

		var tables int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM duckdb_tables() WHERE table_name LIKE 'eg.metrics.%'`).Scan(&tables))
		require.Zero(t, tables)
	})
}
