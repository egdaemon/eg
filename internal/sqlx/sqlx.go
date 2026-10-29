// Package sqlx provides the minimal database/sql-shaped interface that
// genieql-generated code is built against, so callers can pass in any of
// *sql.DB, *sql.Tx, *sql.Conn, etc. without genieql depending on database/sql
// driver specifics.
package sqlx

import (
	"context"
	"database/sql"
	"log"
	"strings"
)

// database/sql returns this as an unexported sentinel (errDBClosed), so it
// cannot be matched with errors.Is; compare by message instead.
const errClosedMsg = "sql: database is closed"

// IgnoreClosed returns nil when err is database/sql's "database is closed"
// error, and err otherwise. Useful for background workers racing DB shutdown.
func IgnoreClosed(err error) error {
	if err != nil && err.Error() == errClosedMsg {
		return nil
	}

	return err
}

// Queryer is the interface genieql-generated code is emitted against, so
// callers can pass either a *sql.DB or a *sql.Tx.
type Queryer interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
	Exec(string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// Columns splits a comma separated list of columns, e.g.) a generated ScannerStaticColumns constant.
func Columns(reference string) []string {
	return strings.Split(reference, ",")
}

// Transactioner is a Queryer that can also begin transactions.
type Transactioner interface {
	Queryer
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

// Row interface for scanning a single row.
type Row interface {
	Scan(dest ...any) error
}

// Debug creates a DebuggingQueryer
func Debug(q Queryer) DebuggingQueryer {
	return DebuggingQueryer{
		Delegate: q,
	}
}

// DebuggingQueryer queryer that prints out the queries being executed.
type DebuggingQueryer struct {
	Delegate Queryer
}

// Query execute a query
func (t DebuggingQueryer) Query(q string, args ...any) (*sql.Rows, error) {
	return t.QueryContext(context.Background(), q, args...)
}

// QueryRow executes a query that returns a single row.
func (t DebuggingQueryer) QueryRow(q string, args ...any) *sql.Row {
	return t.QueryRowContext(context.Background(), q, args...)
}

// Exec executes a statement.
func (t DebuggingQueryer) Exec(q string, args ...any) (sql.Result, error) {
	return t.ExecContext(context.Background(), q, args...)
}

// QueryContext ...
func (t DebuggingQueryer) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	log.Printf("%s:\n%#v\n", q, args)
	return t.Delegate.QueryContext(ctx, q, args...)
}

// QueryRowContext ...
func (t DebuggingQueryer) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	log.Printf("%s:\n%#v\n", q, args)
	return t.Delegate.QueryRowContext(ctx, q, args...)
}

// ExecContext ...
func (t DebuggingQueryer) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	log.Printf("%s:\n%#v\n", q, args)
	return t.Delegate.ExecContext(ctx, q, args...)
}
