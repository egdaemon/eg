// Package duckproxyv2_test holds shared helpers for the driver-level tests
// (driver.*.gen_test.go, server.serve.gen_test.go) -- a black-box suite
// exercising the public sql.Open("duckproxyv2", ...) surface, as opposed
// to the internal package's tests of unexported framing/value-conversion
// functions.
package duckproxyv2_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/duckproxyv2"
)

// startTestServer starts a duckproxyv2.Server backed by a temp-file DuckDB
// database, listening on a temp-dir unix socket, and returns the socket
// path plus the server-side *sql.DB (for inspecting pool stats, e.g.
// db.Stats().InUse in the concurrency test).
func startTestServer(t *testing.T, opts ...duckproxyv2.Option) (socketPath string, serverDB *sql.DB) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	socketPath = filepath.Join(dir, "test.sock")

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(duckdb): %v", err)
	}
	t.Cleanup(func() { db.Close() })

	srv := duckproxyv2.New(db, opts...)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- duckproxyv2.ListenUnix(ctx, socketPath, srv)
	}()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	waitForSocket(t, socketPath)

	return socketPath, db
}

// connect opens a client connection through duckproxyv2's
// database/sql/driver.Driver.
func connect(t *testing.T, socketPath string) *sql.DB {
	t.Helper()

	db, err := sql.Open("duckproxyv2", socketPath)
	if err != nil {
		t.Fatalf("sql.Open(duckproxyv2): %v", err)
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
