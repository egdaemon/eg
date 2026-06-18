// Package sqlx provides the minimal database/sql-shaped interface that
// genieql-generated code is built against, so callers can pass in any of
// *sql.DB, *sql.Tx, *sql.Conn, etc. without genieql depending on database/sql
// driver specifics.
package sqlx

import (
	"context"
	"database/sql"
)

// Queryer interface for executing queries.
type Queryer interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
	Exec(string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// Row interface for scanning a single row.
type Row interface {
	Scan(dest ...any) error
}
