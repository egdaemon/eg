package duckproxy

import (
	"context"

	"github.com/jackc/pgx/v5/pgproto3"
)

// handleQuery implements the simple query protocol: one Query message in,
// a full response (RowDescription/DataRow*/CommandComplete or
// ErrorResponse) plus a trailing ReadyForQuery out. Multi-statement
// batches in a single Query message are not supported -- DuckDB's own
// Prepare rejects anything but exactly one statement, which surfaces here
// as an ordinary ErrorResponse.
func (s *session) handleQuery(ctx context.Context, msg *pgproto3.Query) {
	stmt, err := prepareStatement(ctx, s.dconn, "", msg.String, nil)
	if err != nil {
		if s.tx != txIdle {
			s.tx = txFailed
		}
		s.backend.Send(toErrorResponse(err))
		s.backend.Send(&pgproto3.ReadyForQuery{TxStatus: s.tx.statusByte()})
		return
	}
	defer stmt.close()

	if err := stmt.stmt.Bind(nil); err != nil {
		if s.tx != txIdle {
			s.tx = txFailed
		}
		s.backend.Send(toErrorResponse(err))
		s.backend.Send(&pgproto3.ReadyForQuery{TxStatus: s.tx.statusByte()})
		return
	}

	if stmt.tuples {
		s.backend.Send(buildRowDescription(stmt))
	}

	s.runStatement(ctx, stmt, nil)

	s.backend.Send(&pgproto3.ReadyForQuery{TxStatus: s.tx.statusByte()})
}
