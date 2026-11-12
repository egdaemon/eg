package main

import (
	"context"
	"fmt"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/x/wasi/egdb"
	"github.com/egdaemon/wasinet/wasinet"
	"github.com/jackc/pgx/v5"
)

func ExecQuery(ctx context.Context, o eg.Op) error {
	db, err := egdb.OpenVerify(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, "CREATE OR REPLACE TABLE duckproxy_integration (id INTEGER, name VARCHAR)"); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS duckproxy_integration")

	if _, err := db.ExecContext(ctx, "INSERT INTO duckproxy_integration VALUES (?, ?), (?, ?)", 1, "a", 2, "b"); err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	var count int
	var names string
	if err := db.QueryRowContext(ctx, "SELECT count(*), string_agg(name, ',' ORDER BY id) FROM duckproxy_integration").Scan(&count, &names); err != nil {
		return fmt.Errorf("query: %w", err)
	}

	if count != 2 || names != "a,b" {
		return fmt.Errorf("unexpected result: count=%d names=%q", count, names)
	}

	log.Println("exec/query roundtrip succeeded")
	return nil
}

func Transaction(ctx context.Context, o eg.Op) error {
	db, err := egdb.OpenVerify(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, "CREATE OR REPLACE TABLE duckproxy_integration_tx (id INTEGER)"); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer db.ExecContext(ctx, "DROP TABLE IF EXISTS duckproxy_integration_tx")

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	if _, err = tx.ExecContext(ctx, "INSERT INTO duckproxy_integration_tx VALUES (1)"); err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	if err = tx.Rollback(); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}

	var count int
	if err = db.QueryRowContext(ctx, "SELECT count(*) FROM duckproxy_integration_tx").Scan(&count); err != nil {
		return fmt.Errorf("query: %w", err)
	}

	if count != 0 {
		return fmt.Errorf("rollback did not discard the insert: count=%d", count)
	}

	log.Println("transaction rollback succeeded")
	return nil
}

// PGX exercises the postgres wire frontend from inside the guest, connecting to the unix socket in the runtime
// directory. pgconn derives the socket name (eg.SocketAnalyticsPG) from the directory it is given as the host.
func PGX(ctx context.Context, o eg.Op) error {
	cfg, err := pgx.ParseConfig(fmt.Sprintf("host=%s dbname=analytics sslmode=disable", egunsafe.RuntimeDirectory()))
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	cfg.DialFunc = wasinet.DialContext

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	var answer int
	if err = conn.QueryRow(ctx, "SELECT 41 + 1").Scan(&answer); err != nil {
		return fmt.Errorf("select: %w", err)
	}

	if answer != 42 {
		return fmt.Errorf("unexpected answer: %d", answer)
	}

	if _, err = conn.Exec(ctx, "CREATE OR REPLACE TABLE duckproxy_integration_pgx (id INTEGER, name VARCHAR)"); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer conn.Exec(ctx, "DROP TABLE IF EXISTS duckproxy_integration_pgx")

	if _, err = conn.Exec(ctx, "INSERT INTO duckproxy_integration_pgx VALUES (1, 'a'), (2, 'b')"); err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	var count int
	var names string
	if err = conn.QueryRow(ctx, "SELECT count(*), string_agg(name, ',' ORDER BY id) FROM duckproxy_integration_pgx").Scan(&count, &names); err != nil {
		return fmt.Errorf("query: %w", err)
	}

	if count != 2 || names != "a,b" {
		return fmt.Errorf("unexpected result: count=%d names=%q", count, names)
	}

	log.Println("pgx socket roundtrip succeeded")
	return nil
}

func main() {
	log.SetFlags(log.Lshortfile)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	log.Println("-------------------------- egdb test initiated --------------------------")
	defer log.Println("-------------------------- egdb test completed --------------------------")

	err := eg.Perform(
		ctx,
		ExecQuery,
		Transaction,
		PGX,
	)
	if err != nil {
		log.Fatalln(err)
	}
}
