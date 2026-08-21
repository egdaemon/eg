// Package sqlx contains small helpers for working with database/sql that
// arent provided by the standard library.
package sqlx

import (
	"context"
	"database/sql"
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

// Transactioner is a Queryer that can also begin transactions.
type Transactioner interface {
	Queryer
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}
