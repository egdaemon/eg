package duckproxyserver

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/duckproxy"
)

// handleExec runs a non-tuple-returning statement. Unlike a Postgres-wire
// proxy, the server never has to classify the statement itself -- the
// client already told us it's an Exec by sending this frame, since
// database/sql itself makes that distinction (ExecContext vs QueryContext)
// before our driver.Conn is ever involved.
func (s *session) handleExec(ctx context.Context, req *duckproxy.ExecRequest) error {
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

	return duckproxy.WriteFrame(s.conn, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Result{Result: &duckproxy.ExecResponse{RowsAffected: n}}})
}

// handleQuery runs a tuple-returning statement and streams its result set
// back one duckproxy.RowResponse frame per row, as DuckDB produces them -- it must
// not buffer the whole result set before writing the first row. See the
// package-level streaming requirement.
func (s *session) handleQuery(ctx context.Context, req *duckproxy.QueryRequest) error {
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

	if err := duckproxy.WriteFrame(s.conn, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Columns{Columns: &duckproxy.ColumnsResponse{Names: rows.Columns()}}}); err != nil {
		return err
	}

	dst := make([]driver.Value, len(rows.Columns()))
	for {
		err := rows.Next(dst)
		if err == io.EOF {
			return duckproxy.WriteFrame(s.conn, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Done{Done: &duckproxy.DoneResponse{}}})
		}
		if err != nil {
			return s.sendError(err)
		}

		values := make([]*duckproxy.Value, len(dst))
		for i, v := range dst {
			pv, err := toProtoValue(v)
			if err != nil {
				return s.sendError(err)
			}
			values[i] = pv
		}

		if err := duckproxy.WriteFrame(s.conn, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Row{Row: &duckproxy.RowResponse{Values: values}}}); err != nil {
			return err
		}
	}
}

// handleDescribe prepares req.Sql without executing it, purely to report
// parameter/result type info -- needed by Postgres-wire frontends
// (duckpgwire) for ParameterDescription/RowDescription, which neither end
// of this protocol otherwise needs.
func (s *session) handleDescribe(ctx context.Context, req *duckproxy.DescribeRequest) error {
	ctx, cancel := s.statementContext(ctx)
	defer cancel()

	driverStmt, err := s.dconn.PrepareContext(ctx, req.GetSql())
	if err != nil {
		return s.sendError(err)
	}
	stmt := driverStmt.(*duckdb.Stmt)
	defer stmt.Close()

	st, err := stmt.StatementType()
	if err != nil {
		return s.sendError(err)
	}
	tuples := isTupleReturning(st)

	numInput := stmt.NumInput()
	params := make([]*duckproxy.TypeMetadata, numInput)
	for i := range numInput {
		pt, err := stmt.ParamType(i + 1)
		if err != nil {
			pt = duckdb.TYPE_INVALID
		}
		params[i] = &duckproxy.TypeMetadata{Type: uint32(pt)}
	}

	var columns []*duckproxy.TypeMetadata
	if tuples {
		colCount, err := stmt.ColumnCount()
		if err != nil {
			return s.sendError(err)
		}
		columns = make([]*duckproxy.TypeMetadata, colCount)
		for i := range colCount {
			ct, err := stmt.ColumnType(i)
			if err != nil {
				return s.sendError(err)
			}
			cn, err := stmt.ColumnName(i)
			if err != nil {
				return s.sendError(err)
			}
			columns[i] = &duckproxy.TypeMetadata{Name: cn, Type: uint32(ct)}
		}
	}

	return duckproxy.WriteFrame(s.conn, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Describe{Describe: &duckproxy.DescribeResponse{
		Tuples:        tuples,
		Params:        params,
		Columns:       columns,
		StatementType: uint32(st),
	}}})
}

func isTupleReturning(st duckdb.StmtType) bool {
	switch st {
	case duckdb.STATEMENT_TYPE_SELECT, duckdb.STATEMENT_TYPE_EXPLAIN, duckdb.STATEMENT_TYPE_CALL, duckdb.STATEMENT_TYPE_PRAGMA:
		return true
	default:
		return false
	}
}

func (s *session) handleBegin(ctx context.Context) error {
	if s.tx != nil {
		return s.sendError(errors.New("duckproxy: transaction already in progress"))
	}

	tx, err := s.dconn.BeginTx(ctx, driver.TxOptions{})
	if err != nil {
		return s.sendError(err)
	}

	s.tx = tx
	return duckproxy.WriteFrame(s.conn, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Ok{Ok: &duckproxy.OkResponse{}}})
}

func (s *session) handleCommit() error {
	if s.tx == nil {
		return s.sendError(errors.New("duckproxy: no transaction in progress"))
	}

	tx := s.tx
	s.tx = nil

	if err := tx.Commit(); err != nil {
		return s.sendError(err)
	}
	return duckproxy.WriteFrame(s.conn, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Ok{Ok: &duckproxy.OkResponse{}}})
}

func (s *session) handleRollback() error {
	if s.tx == nil {
		return s.sendError(errors.New("duckproxy: no transaction in progress"))
	}

	tx := s.tx
	s.tx = nil

	if err := tx.Rollback(); err != nil {
		return s.sendError(err)
	}
	return duckproxy.WriteFrame(s.conn, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Ok{Ok: &duckproxy.OkResponse{}}})
}
