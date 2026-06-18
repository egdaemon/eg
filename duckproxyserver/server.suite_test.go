// Package server_test holds shared helpers for the server-level tests
// (server.serve.gen_test.go) -- a black-box suite exercising the public
// sql.Open("duckproxy", ...) surface against a real server.Server, as
// opposed to duckproxy's own driver-level test suite.
package duckproxyserver_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/duckproxyserver"
)

// startTestServer starts a server.Server backed by a temp-file DuckDB
// database, listening on a temp-dir unix socket, and returns the socket
// path plus the server-side *sql.DB (for inspecting pool stats, e.g.
// db.Stats().InUse in the concurrency test).
func startTestServer(t *testing.T, opts ...duckproxyserver.Option) (socketPath string, serverDB *sql.DB) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	socketPath = filepath.Join(dir, "test.sock")

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(duckdb): %v", err)
	}
	t.Cleanup(func() { db.Close() })

	srv := duckproxyserver.New(db, opts...)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- duckproxyserver.ListenUnix(ctx, socketPath, srv)
	}()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	waitForSocket(t, socketPath)

	return socketPath, db
}

// connect opens a client connection through duckproxy's
// database/sql/driver.Driver.
func connect(t *testing.T, socketPath string) *sql.DB {
	t.Helper()

	db, err := sql.Open("duckproxy", socketPath)
	if err != nil {
		t.Fatalf("sql.Open(duckproxy): %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return db
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("socket %s did not appear in time", path)
}
