package duckproxyv2

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
)

// handleExec runs a non-tuple-returning statement. Unlike a Postgres-wire
// proxy, the server never has to classify the statement itself -- the
// client already told us it's an Exec by sending this frame, since
// database/sql itself makes that distinction (ExecContext vs QueryContext)
// before our driver.Conn is ever involved.
func (s *session) handleExec(ctx context.Context, req *ExecRequest) error {
	args, err := toNamedValues(req.GetArgs())
	if err != nil {
		return s.sendError(err)
	}

	ctx, cancel := s.statementContext(ctx)
	defer cancel()

	res, err := s.dconn.ExecContext(ctx, req.GetSql(), args)
	if err != nil {
		return s.sendError(err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		n = 0
	}

	return writeFrame(s.conn, &ServerFrame{Body: &ServerFrame_Result{Result: &ExecResponse{RowsAffected: n}}})
}

// handleQuery runs a tuple-returning statement and streams its result set
// back one RowResponse frame per row, as DuckDB produces them -- it must
// not buffer the whole result set before writing the first row. See the
// package-level streaming requirement.
func (s *session) handleQuery(ctx context.Context, req *QueryRequest) error {
	args, err := toNamedValues(req.GetArgs())
	if err != nil {
		return s.sendError(err)
	}

	ctx, cancel := s.statementContext(ctx)
	defer cancel()

	rows, err := s.dconn.QueryContext(ctx, req.GetSql(), args)
	if err != nil {
		return s.sendError(err)
	}
	defer rows.Close()

	if err := writeFrame(s.conn, &ServerFrame{Body: &ServerFrame_Columns{Columns: &ColumnsResponse{Names: rows.Columns()}}}); err != nil {
		return err
	}

	dst := make([]driver.Value, len(rows.Columns()))
	for {
		err := rows.Next(dst)
		if err == io.EOF {
			return writeFrame(s.conn, &ServerFrame{Body: &ServerFrame_Done{Done: &DoneResponse{}}})
		}
		if err != nil {
			return s.sendError(err)
		}

		values := make([]*Value, len(dst))
		for i, v := range dst {
			pv, err := toProtoValue(v)
			if err != nil {
				return s.sendError(err)
			}
			values[i] = pv
		}

		if err := writeFrame(s.conn, &ServerFrame{Body: &ServerFrame_Row{Row: &RowResponse{Values: values}}}); err != nil {
			return err
		}
	}
}

func (s *session) handleBegin(ctx context.Context) error {
	if s.tx != nil {
		return s.sendError(errors.New("duckproxyv2: transaction already in progress"))
	}

	tx, err := s.dconn.BeginTx(ctx, driver.TxOptions{})
	if err != nil {
		return s.sendError(err)
	}

	s.tx = tx
	return writeFrame(s.conn, &ServerFrame{Body: &ServerFrame_Ok{Ok: &OkResponse{}}})
}

func (s *session) handleCommit() error {
	if s.tx == nil {
		return s.sendError(errors.New("duckproxyv2: no transaction in progress"))
	}

	tx := s.tx
	s.tx = nil

	if err := tx.Commit(); err != nil {
		return s.sendError(err)
	}
	return writeFrame(s.conn, &ServerFrame{Body: &ServerFrame_Ok{Ok: &OkResponse{}}})
}

func (s *session) handleRollback() error {
	if s.tx == nil {
		return s.sendError(errors.New("duckproxyv2: no transaction in progress"))
	}

	tx := s.tx
	s.tx = nil

	if err := tx.Rollback(); err != nil {
		return s.sendError(err)
	}
	return writeFrame(s.conn, &ServerFrame{Body: &ServerFrame_Ok{Ok: &OkResponse{}}})
}
