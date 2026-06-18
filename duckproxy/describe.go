package duckproxy

import (
	"context"
	"database/sql"
	"errors"
)

// DescribedType pairs a DuckDB type with whatever identifies its slot --
// the Go-side counterpart of the wire message TypeMetadata (named
// differently to avoid colliding with the generated type of that name).
// Name is empty for parameters, which are positional ($1, $2, ...), not
// named. Type is the raw duckdb.Type enum value as a uint32 rather than
// duckdb.Type itself -- this package stays free of duckdb-go (and its cgo
// bindings) so it can build for GOOS=wasip1; callers that want the
// strongly-typed enum (e.g. duckpgwire, which already depends on
// duckdb-go) can convert via duckdb.Type(d.Type).
type DescribedType struct {
	Name string
	Type uint32
}

// DescribeResult is the outcome of Describe: enough to build a Postgres
// ParameterDescription/RowDescription without having executed anything.
type DescribeResult struct {
	Tuples        bool
	Params        []DescribedType
	Columns       []DescribedType
	StatementType uint32 // raw duckdb.StmtType enum value, e.g. duckdb.STATEMENT_TYPE_INSERT
}

// Describe prepares query against the duckproxy server sqlConn is
// connected to, without executing it, and reports its parameter/result
// type info. This is the one piece of this protocol that exists purely
// for Postgres-wire frontends (duckpgwire) -- neither end otherwise needs
// an OID/type system, since both already speak DuckDB's native types
// directly. Mirrors duckdb-go's own GetTableNames/ConnId pattern
// (vendor/.../connection.go) of a package-level helper that does the
// (*sql.Conn).Raw + type-assert internally so callers never see *conn.
func Describe(ctx context.Context, sqlConn *sql.Conn, query string) (DescribeResult, error) {
	var result DescribeResult

	err := sqlConn.Raw(func(driverConn any) error {
		c := driverConn.(*conn)
		defer watchCancel(ctx, c.c)()

		if err := WriteFrame(c.c, &ClientFrame{Body: &ClientFrame_Describe{Describe: &DescribeRequest{Sql: query}}}); err != nil {
			return err
		}

		var resp ServerFrame
		if err := ReadFrame(c.c, &resp); err != nil {
			return err
		}

		switch body := resp.GetBody().(type) {
		case *ServerFrame_Describe:
			result = fromDescribeResponse(body.Describe)
			return nil
		case *ServerFrame_Error:
			return errors.New(body.Error.GetMessage())
		default:
			return errors.New("duckproxy: unexpected response to Describe")
		}
	})

	return result, err
}

func fromDescribeResponse(resp *DescribeResponse) DescribeResult {
	params := make([]DescribedType, len(resp.GetParams()))
	for i, p := range resp.GetParams() {
		params[i] = DescribedType{Name: p.GetName(), Type: p.GetType()}
	}

	columns := make([]DescribedType, len(resp.GetColumns()))
	for i, c := range resp.GetColumns() {
		columns[i] = DescribedType{Name: c.GetName(), Type: c.GetType()}
	}

	return DescribeResult{
		Tuples:        resp.GetTuples(),
		Params:        params,
		Columns:       columns,
		StatementType: resp.GetStatementType(),
	}
}
