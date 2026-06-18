package duckpgwire

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgproto3"
)

// execer abstracts over *sql.Conn and *sql.Tx -- whichever the session
// should currently route statement execution through. Both satisfy this
// directly; no wrapping needed.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// execer returns the *sql.Tx if a transaction is open, otherwise the
// session's underlying *sql.Conn.
func (s *session) execer() execer {
	if s.dbTx != nil {
		return s.dbTx
	}
	return s.sqlConn
}

// runStatement executes stmt with args (already decoded, for the extended
// protocol; the simple query protocol passes nil, since it never carries
// parameters) and sends its result -- DataRow*/CommandComplete or
// ErrorResponse -- to the client. p is nil for the simple query protocol,
// where every column is always text.
func (s *session) runStatement(ctx context.Context, stmt *preparedStatement, args []any, p *portal) {
	kw := classifyTxKeyword(stmt.query)

	if s.tx == txFailed && kw != txKeywordRollback && kw != txKeywordCommit {
		s.backend.Send(errorResponse(sqlStateInFailedTransaction, errInFailedTransaction.Error()))
		return
	}

	// Postgres silently turns a COMMIT into a ROLLBACK once the
	// transaction block has already failed.
	if s.tx == txFailed && kw == txKeywordCommit {
		kw = txKeywordRollback
	}

	var (
		tag string
		err error
	)
	switch kw {
	case txKeywordBegin:
		tag, err = s.runBegin(ctx)
	case txKeywordCommit:
		tag, err = s.runCommit()
	case txKeywordRollback:
		tag, err = s.runRollback()
	default:
		sctx, cancel := s.statementContext(ctx)
		defer func() {
			cancel()
			s.setCancel(nil)
		}()

		if stmt.tuples {
			tag, err = s.runQuery(sctx, stmt, args, p)
		} else {
			tag, err = s.runExec(sctx, stmt, args)
		}
	}

	if err != nil {
		if s.tx != txIdle {
			s.tx = txFailed
		}
		s.backend.Send(toErrorResponse(err))
		return
	}

	switch kw {
	case txKeywordBegin:
		s.tx = txInTransaction
	case txKeywordCommit, txKeywordRollback:
		s.tx = txIdle
	}

	s.backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
}

func (s *session) runBegin(ctx context.Context) (string, error) {
	if s.dbTx != nil {
		return "", errors.New("duckpgwire: BEGIN issued while already inside a transaction")
	}

	tx, err := s.sqlConn.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}

	s.dbTx = tx
	return "BEGIN", nil
}

func (s *session) runCommit() (string, error) {
	if s.dbTx == nil {
		// A bare COMMIT outside a transaction: Postgres warns and no-ops
		// rather than erroring.
		return "COMMIT", nil
	}

	tx := s.dbTx
	s.dbTx = nil

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return "COMMIT", nil
}

func (s *session) runRollback() (string, error) {
	if s.dbTx == nil {
		return "ROLLBACK", nil
	}

	tx := s.dbTx
	s.dbTx = nil

	if err := tx.Rollback(); err != nil {
		return "", err
	}
	return "ROLLBACK", nil
}

func (s *session) runQuery(ctx context.Context, stmt *preparedStatement, args []any, p *portal) (string, error) {
	rows, err := s.execer().QueryContext(ctx, stmt.query, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	dst := make([]any, len(stmt.columnOIDs))
	scanArgs := make([]any, len(dst))
	for i := range dst {
		scanArgs[i] = &dst[i]
	}

	n := 0
	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return "", err
		}
		n++

		values := make([][]byte, len(dst))
		for i, v := range dst {
			var format int16
			if p != nil {
				format = p.formatFor(i)
			}
			b, err := encodeValue(stmt.columnOIDs[i], format, v)
			if err != nil {
				return "", err
			}
			values[i] = b
		}
		s.backend.Send(&pgproto3.DataRow{Values: values})
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	return fmt.Sprintf("SELECT %d", n), nil
}

func (s *session) runExec(ctx context.Context, stmt *preparedStatement, args []any) (string, error) {
	res, err := s.execer().ExecContext(ctx, stmt.query, args...)
	if err != nil {
		return "", err
	}
	n, err := res.RowsAffected()
	if err != nil {
		n = 0
	}
	return commandTag(stmt.stmtType, n), nil
}
