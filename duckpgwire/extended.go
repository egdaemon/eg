package duckpgwire

import (
	"context"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
)

// resolveFormat applies Postgres's format-code rules: zero codes means
// text for everything, one code applies to everything, otherwise it's one
// code per item.
func resolveFormat(codes []int16, i int) int16 {
	switch len(codes) {
	case 0:
		return 0
	case 1:
		return codes[0]
	default:
		return codes[i]
	}
}

func (s *session) handleParse(ctx context.Context, msg *pgproto3.Parse) {
	delete(s.statements, msg.Name)

	stmt, err := prepareStatement(ctx, s.sqlConn, msg.Name, msg.Query, msg.ParameterOIDs)
	if err != nil {
		if s.tx != txIdle {
			s.tx = txFailed
		}
		s.backend.Send(toErrorResponse(err))
		return
	}

	s.statements[msg.Name] = stmt
	s.backend.Send(&pgproto3.ParseComplete{})
}

func (s *session) handleBind(msg *pgproto3.Bind) {
	stmt, ok := s.statements[msg.PreparedStatement]
	if !ok {
		s.backend.Send(errorResponse(sqlStateInternalError, "unknown prepared statement: "+msg.PreparedStatement))
		return
	}

	args := make([]any, len(msg.Parameters))
	for i, raw := range msg.Parameters {
		oid := pgTextOIDFallback(stmt.paramOIDs, i)
		format := resolveFormat(msg.ParameterFormatCodes, i)

		val, err := decodeParam(oid, format, raw)
		if err != nil {
			if s.tx != txIdle {
				s.tx = txFailed
			}
			s.backend.Send(toErrorResponse(err))
			return
		}
		args[i] = val
	}

	s.portals[msg.DestinationPortal] = &portal{
		name:          msg.DestinationPortal,
		stmt:          stmt,
		args:          args,
		resultFormats: msg.ResultFormatCodes,
	}
	s.backend.Send(&pgproto3.BindComplete{})
}

func (s *session) handleDescribe(msg *pgproto3.Describe) {
	switch msg.ObjectType {
	case 'S':
		stmt, ok := s.statements[msg.Name]
		if !ok {
			s.backend.Send(errorResponse(sqlStateInternalError, "unknown prepared statement: "+msg.Name))
			return
		}
		s.backend.Send(&pgproto3.ParameterDescription{ParameterOIDs: stmt.paramOIDs})
		if stmt.tuples {
			s.backend.Send(buildRowDescription(stmt))
		} else {
			s.backend.Send(&pgproto3.NoData{})
		}
	case 'P':
		p, ok := s.portals[msg.Name]
		if !ok {
			s.backend.Send(errorResponse(sqlStateInternalError, "unknown portal: "+msg.Name))
			return
		}
		if p.stmt.tuples {
			s.backend.Send(buildRowDescription(p.stmt))
		} else {
			s.backend.Send(&pgproto3.NoData{})
		}
	}
}

func (s *session) handleExecute(ctx context.Context, msg *pgproto3.Execute) {
	p, ok := s.portals[msg.Portal]
	if !ok {
		s.backend.Send(errorResponse(sqlStateInternalError, "unknown portal: "+msg.Portal))
		return
	}

	s.runStatement(ctx, p.stmt, p.args, p)
}

func (s *session) handleSync() {
	s.backend.Send(&pgproto3.ReadyForQuery{TxStatus: s.tx.statusByte()})
}

func (s *session) handleClose(msg *pgproto3.Close) {
	switch msg.ObjectType {
	case 'S':
		delete(s.statements, msg.Name)
	case 'P':
		delete(s.portals, msg.Name)
	}
	s.backend.Send(&pgproto3.CloseComplete{})
}

// pgTextOIDFallback returns oids[i], or the text OID if i is out of range
// -- defensive against a malformed Bind sending more parameters than the
// statement declared.
func pgTextOIDFallback(oids []uint32, i int) uint32 {
	if i < len(oids) {
		return oids[i]
	}
	return pgtype.TextOID
}
