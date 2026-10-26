package fficoverage

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/interp/events"
	"github.com/stretchr/testify/require"
)

// reports are sent over the analytics socket, and every Report dials a fresh connection.
// more sequential full batches than the server's connection cap must all complete.
func TestReportThroughProxy(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "analytics.socket")

	db, err := sql.Open("duckdb", filepath.Join(dir, "analytics.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	pctx, pdone := context.WithTimeout(context.Background(), 10*time.Second)
	defer pdone()
	require.NoError(t, events.PrepareDB(pctx, db))

	sctx, stop := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() {
		served <- duckproxyserver.ListenUnix(sctx, socketPath, duckproxyserver.New(db))
	}()
	t.Cleanup(func() {
		stop()
		<-served
	})

	require.Eventually(t, func() bool {
		c, err := sql.Open("duckproxy", socketPath)
		if err != nil {
			return false
		}
		defer c.Close()
		return c.PingContext(context.Background()) == nil
	}, 2*time.Second, 10*time.Millisecond)

	const (
		batches = 12
		size    = 128
	)

	for i := range batches {
		t.Run(fmt.Sprintf("batch_%d", i), func(t *testing.T) {
			ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()

			batch := make([]*coverage.Report, 0, size)
			for j := range size {
				batch = append(batch, &coverage.Report{Path: fmt.Sprintf("p%d.go", i), Fnname: fmt.Sprintf("Fn%d", j), Hits: int64(j), Statements: 50, Branches: 50})
			}

			client := sql.OpenDB(duckproxy.NewConnector(socketPath, func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			}))
			defer client.Close()

			require.NoError(t, report(ctx, client, batch...))
		})
	}

	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM 'eg.metrics.coverage'`).Scan(&n))
	require.Equal(t, batches*size, n)
}
