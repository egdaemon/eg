package duckproxy

import (
	"context"
	"database/sql"
	"errors"

	duckdb "github.com/duckdb/duckdb-go/v2"
)

// DescribedType pairs a DuckDB type with whatever identifies its slot --
// the Go-side counterpart of the wire message TypeMetadata (named
// differently to avoid colliding with the generated type of that name).
// Name is empty for parameters, which are positional ($1, $2, ...), not
// named.
type DescribedType struct {
	Name string
	Type duckdb.Type
}

// DescribeResult is the outcome of Describe: enough to build a Postgres
// ParameterDescription/RowDescription without having executed anything.
type DescribeResult struct {
	Tuples        bool
	Params        []DescribedType
	Columns       []DescribedType
	StatementType duckdb.StmtType // e.g. duckdb.STATEMENT_TYPE_INSERT; a distinct enum from DescribedType.Type
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

		if err := writeFrame(c.c, &ClientFrame{Body: &ClientFrame_Describe{Describe: &DescribeRequest{Sql: query}}}); err != nil {
			return err
		}

		var resp ServerFrame
		if err := readFrame(c.c, &resp); err != nil {
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
		params[i] = DescribedType{Name: p.GetName(), Type: duckdb.Type(p.GetType())}
	}

	columns := make([]DescribedType, len(resp.GetColumns()))
	for i, c := range resp.GetColumns() {
		columns[i] = DescribedType{Name: c.GetName(), Type: duckdb.Type(c.GetType())}
	}

	return DescribeResult{
		Tuples:        resp.GetTuples(),
		Params:        params,
		Columns:       columns,
		StatementType: duckdb.StmtType(resp.GetStatementType()),
	}
}
