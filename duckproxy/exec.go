package duckproxy

import (
	"context"
	"database/sql/driver"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgproto3"
)

// runStatement executes stmt (already Bound, for the extended protocol; the
// simple query protocol Binds with zero arguments itself before calling
// this) and sends its result -- DataRow*/CommandComplete or ErrorResponse
// -- to the client. p is nil for the simple query protocol, where every
// column is always text.
func (s *session) runStatement(ctx context.Context, stmt *preparedStatement, p *portal) {
	kw := classifyTxKeyword(stmt.query)

	if s.tx == txFailed && kw != txKeywordRollback && kw != txKeywordCommit {
		s.backend.Send(errorResponse(sqlStateInFailedTransaction, errInFailedTransaction.Error()))
		return
	}

	// Postgres silently turns a COMMIT into a ROLLBACK once the
	// transaction block has already failed.
	if s.tx == txFailed && kw == txKeywordCommit {
		if _, err := s.dconn.ExecContext(ctx, "ROLLBACK", nil); err != nil {
			s.backend.Send(toErrorResponse(err))
			return
		}
		s.tx = txIdle
		s.backend.Send(&pgproto3.CommandComplete{CommandTag: []byte("ROLLBACK")})
		return
	}

	ctx, cancel := s.statementContext(ctx)
	defer func() {
		cancel()
		s.setCancel(nil)
	}()

	var (
		tag string
		err error
	)
	if stmt.tuples {
		tag, err = s.runQuery(ctx, stmt, p)
	} else {
		tag, err = s.runExec(ctx, stmt)
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
		tag = "BEGIN"
	case txKeywordCommit:
		s.tx = txIdle
		tag = "COMMIT"
	case txKeywordRollback:
		s.tx = txIdle
		tag = "ROLLBACK"
	}

	s.backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(tag)})
}

func (s *session) runQuery(ctx context.Context, stmt *preparedStatement, p *portal) (string, error) {
	rows, err := stmt.stmt.QueryBound(ctx)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	dst := make([]driver.Value, len(stmt.columnOIDs))
	n := 0
	for {
		err := rows.Next(dst)
		if err == io.EOF {
			break
		}
		if err != nil {
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

	return fmt.Sprintf("SELECT %d", n), nil
}

func (s *session) runExec(ctx context.Context, stmt *preparedStatement) (string, error) {
	res, err := stmt.stmt.ExecBound(ctx)
	if err != nil {
		return "", err
	}
	n, err := res.RowsAffected()
	if err != nil {
		n = 0
	}
	return commandTag(stmt.stmtType, n), nil
}
