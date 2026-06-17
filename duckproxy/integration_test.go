package duckproxy_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/jackc/pgx/v5"
)

func startTestServer(t *testing.T, opts ...duckproxy.Option) (dir string, db *sql.DB) {
	t.Helper()

	dir = t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	srv := duckproxy.New(db, opts...)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- duckproxy.ListenUnix(ctx, filepath.Join(dir, ".s.PGSQL.5432"), srv)
	}()
	t.Cleanup(func() {
		cancel()
		<-errCh
	})

	waitForSocket(t, filepath.Join(dir, ".s.PGSQL.5432"))

	return dir, db
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

func connect(t *testing.T, dir string, simple bool) *pgx.Conn {
	t.Helper()

	cfg, err := pgx.ParseConfig(fmt.Sprintf("host=%s port=5432 database=test sslmode=disable", dir))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if simple {
		cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}

	conn, err := pgx.ConnectConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ConnectConfig: %v", err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

func TestSimpleProtocol(t *testing.T) {
	dir, _ := startTestServer(t)
	conn := connect(t, dir, true)
	ctx := context.Background()

	if _, err := conn.Exec(ctx, "CREATE TABLE t (id INTEGER, name VARCHAR)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO t VALUES (1, 'a')"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	rows, err := conn.Query(ctx, "SELECT id, name FROM t")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int32
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if id != 1 || name != "a" {
			t.Errorf("got (%d, %s), want (1, a)", id, name)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d rows, want 1", count)
	}
}

func TestExtendedProtocol(t *testing.T) {
	dir, _ := startTestServer(t)
	conn := connect(t, dir, false)
	ctx := context.Background()

	if _, err := conn.Exec(ctx, "CREATE TABLE t2 (id INTEGER, name VARCHAR)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO t2 VALUES ($1, $2)", 1, "alice"); err != nil {
		t.Fatalf("insert alice: %v", err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO t2 VALUES ($1, $2)", 2, "bob"); err != nil {
		t.Fatalf("insert bob: %v", err)
	}

	var name string
	if err := conn.QueryRow(ctx, "SELECT name FROM t2 WHERE id = $1", 2).Scan(&name); err != nil {
		t.Fatalf("query row: %v", err)
	}
	if name != "bob" {
		t.Errorf("got %q, want %q", name, "bob")
	}
}

func TestTransaction(t *testing.T) {
	dir, _ := startTestServer(t)
	conn := connect(t, dir, false)
	ctx := context.Background()

	if _, err := conn.Exec(ctx, "CREATE TABLE t3 (id INTEGER)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO t3 VALUES (1)"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var n int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM t3").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("got %d rows after commit, want 1", n)
	}

	tx2, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin 2: %v", err)
	}
	if _, err := tx2.Exec(ctx, "INSERT INTO t3 VALUES (2)"); err != nil {
		t.Fatalf("insert 2: %v", err)
	}
	if err := tx2.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if err := conn.QueryRow(ctx, "SELECT count(*) FROM t3").Scan(&n); err != nil {
		t.Fatalf("count after rollback: %v", err)
	}
	if n != 1 {
		t.Errorf("got %d rows after rollback, want 1", n)
	}
}
