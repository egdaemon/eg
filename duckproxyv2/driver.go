package duckproxyv2

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net"
	"reflect"

	duckdb "github.com/duckdb/duckdb-go/v2"
)

func init() {
	sql.Register("duckproxyv2", &Driver{})
}

// Driver implements database/sql/driver.Driver. Open's dsn is the unix
// socket path a Server is listening on.
type Driver struct{}

func (d *Driver) Open(dsn string) (driver.Conn, error) {
	c, err := net.Dial("unix", dsn)
	if err != nil {
		return nil, err
	}
	return &conn{c: c}, nil
}

type conn struct {
	c net.Conn
}

func (c *conn) Close() error {
	return c.c.Close()
}

func (c *conn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *conn) BeginTx(ctx context.Context, _ driver.TxOptions) (driver.Tx, error) {
	defer watchCancel(ctx, c.c)()

	if err := writeFrame(c.c, &ClientFrame{Body: &ClientFrame_Begin{Begin: &BeginRequest{}}}); err != nil {
		return nil, err
	}
	if err := c.readOK(); err != nil {
		return nil, err
	}
	return &tx{c: c}, nil
}

// watchCancel closes c if ctx is cancelled before the returned stop func
// is called. There is no proactive mid-flight cancellation in this
// protocol (no Postgres-style out-of-band CancelRequest connection): a
// cancelled context simply takes down the connection, which the server
// notices on its next I/O attempt; WithStatementTimeout on the Server
// remains the practical bound on a runaway statement. database/sql/driver
// gives Rows.Next no context at all, so this only covers Exec/Query/Begin
// themselves, not per-row iteration -- the same limitation every
// database/sql driver has.
func watchCancel(ctx context.Context, c net.Conn) (stop func()) {
	if ctx.Done() == nil {
		return func() {}
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			c.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return &stmt{c: c, query: query}, nil
}

func (c *conn) PrepareContext(_ context.Context, query string) (driver.Stmt, error) {
	return &stmt{c: c, query: query}, nil
}

// ExecContext implements driver.ExecerContext, covering every ad hoc
// db.ExecContext call without a Prepare round trip.
func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	defer watchCancel(ctx, c.c)()

	params, err := toParams(args)
	if err != nil {
		return nil, err
	}

	if err := writeFrame(c.c, &ClientFrame{Body: &ClientFrame_Exec{Exec: &ExecRequest{Sql: query, Args: params}}}); err != nil {
		return nil, err
	}

	var resp ServerFrame
	if err := readFrame(c.c, &resp); err != nil {
		return nil, err
	}

	switch body := resp.GetBody().(type) {
	case *ServerFrame_Result:
		return execResult{rowsAffected: body.Result.GetRowsAffected()}, nil
	case *ServerFrame_Error:
		return nil, errors.New(body.Error.GetMessage())
	default:
		return nil, errors.New("duckproxyv2: unexpected response to Exec")
	}
}

// QueryContext implements driver.QueryerContext, covering every ad hoc
// db.QueryContext/db.QueryRowContext call without a Prepare round trip.
func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	defer watchCancel(ctx, c.c)()

	params, err := toParams(args)
	if err != nil {
		return nil, err
	}

	if err := writeFrame(c.c, &ClientFrame{Body: &ClientFrame_Query{Query: &QueryRequest{Sql: query, Args: params}}}); err != nil {
		return nil, err
	}

	var resp ServerFrame
	if err := readFrame(c.c, &resp); err != nil {
		return nil, err
	}

	switch body := resp.GetBody().(type) {
	case *ServerFrame_Columns:
		return &rows{c: c, columns: body.Columns.GetNames()}, nil
	case *ServerFrame_Error:
		return nil, errors.New(body.Error.GetMessage())
	default:
		return nil, errors.New("duckproxyv2: unexpected response to Query")
	}
}

// CheckNamedValue allows the same richer set of Go types toProtoValue
// supports (mirroring duckdb-go's own Conn.CheckNamedValue) to pass
// through unconverted; everything else falls back to database/sql's
// default conversion (to int64/float64/bool/[]byte/string/time.Time),
// which toProtoValue already handles.
func (c *conn) CheckNamedValue(nv *driver.NamedValue) error {
	switch nv.Value.(type) {
	case duckdb.Interval, duckdb.Decimal:
		return nil
	}

	switch reflect.ValueOf(nv.Value).Kind() {
	case reflect.Interface, reflect.Slice, reflect.Map, reflect.Array:
		return nil
	}

	return driver.ErrSkip
}

func (c *conn) readOK() error {
	var resp ServerFrame
	if err := readFrame(c.c, &resp); err != nil {
		return err
	}

	switch body := resp.GetBody().(type) {
	case *ServerFrame_Ok:
		return nil
	case *ServerFrame_Error:
		return errors.New(body.Error.GetMessage())
	default:
		return errors.New("duckproxyv2: unexpected response")
	}
}

func toParams(args []driver.NamedValue) ([]*Param, error) {
	params := make([]*Param, len(args))
	for i, a := range args {
		v, err := toProtoValue(a.Value)
		if err != nil {
			return nil, err
		}
		params[i] = &Param{Ordinal: int32(a.Ordinal), Name: a.Name, Value: v}
	}
	return params, nil
}

type execResult struct {
	rowsAffected int64
}

func (r execResult) LastInsertId() (int64, error) {
	return 0, errors.New("duckproxyv2: LastInsertId not supported")
}

func (r execResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

// stmt is a thin wrapper that delegates to the same one-shot frame round
// trip as conn's ExecerContext/QueryerContext -- there is no server-side
// prepared-statement handle or name, since no current caller needs
// cross-call statement reuse and DuckDB's own Prepare is cheap.
type stmt struct {
	c     *conn
	query string
}

func (s *stmt) Close() error  { return nil }
func (s *stmt) NumInput() int { return -1 }

func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.ExecContext(context.Background(), valuesToNamedValues(args))
}

func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return s.c.ExecContext(ctx, s.query, args)
}

func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.QueryContext(context.Background(), valuesToNamedValues(args))
}

func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return s.c.QueryContext(ctx, s.query, args)
}

func valuesToNamedValues(values []driver.Value) []driver.NamedValue {
	args := make([]driver.NamedValue, len(values))
	for i, v := range values {
		args[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return args
}

// rows streams a query's result set. Next reads exactly one frame per
// call -- it must not pre-read or buffer the result set -- so that any
// `for rows.Next() { ... }` loop gets the same bounded-memory, incremental
// iteration it would get from a direct embedded DuckDB connection.
type rows struct {
	c       *conn
	columns []string
	done    bool
}

func (r *rows) Columns() []string {
	return r.columns
}

// Close drains any remaining frames from an abandoned (not fully read)
// result set, so the connection -- which database/sql will hand to a
// later caller -- isn't left with leftover RowResponse/Done/Error frames
// sitting unread on the wire ahead of that caller's next request.
func (r *rows) Close() error {
	dst := make([]driver.Value, len(r.columns))
	for !r.done {
		if err := r.Next(dst); err != nil {
			break
		}
	}
	return nil
}

func (r *rows) Next(dst []driver.Value) error {
	if r.done {
		return io.EOF
	}

	var resp ServerFrame
	if err := readFrame(r.c.c, &resp); err != nil {
		r.done = true
		return err
	}

	switch body := resp.GetBody().(type) {
	case *ServerFrame_Row:
		values := body.Row.GetValues()
		for i, v := range values {
			gv, err := fromProtoValue(v)
			if err != nil {
				return err
			}
			dst[i] = gv
		}
		return nil
	case *ServerFrame_Done:
		r.done = true
		return io.EOF
	case *ServerFrame_Error:
		r.done = true
		return errors.New(body.Error.GetMessage())
	default:
		r.done = true
		return errors.New("duckproxyv2: unexpected response during row streaming")
	}
}

type tx struct {
	c *conn
}

func (t *tx) Commit() error {
	if err := writeFrame(t.c.c, &ClientFrame{Body: &ClientFrame_Commit{Commit: &CommitRequest{}}}); err != nil {
		return err
	}
	return t.c.readOK()
}

func (t *tx) Rollback() error {
	if err := writeFrame(t.c.c, &ClientFrame{Body: &ClientFrame_Rollback{Rollback: &RollbackRequest{}}}); err != nil {
		return err
	}
	return t.c.readOK()
}
