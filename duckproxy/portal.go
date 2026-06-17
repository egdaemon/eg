package duckproxy

import (
	"context"
	"fmt"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
)

// preparedStatement is created by a Parse message (or, for the simple query
// protocol, synthesized for the single statement in a Query message). It
// may be Bound into a portal more than once, but duckproxy always fully
// drains and closes a portal's rows within the Execute call that follows
// its Bind, so only one portal per statement is ever open at a time --
// holding multiple concurrently-open portals against the same prepared
// statement is not supported.
type preparedStatement struct {
	name        string
	query       string
	stmt        *duckdb.Stmt
	stmtType    duckdb.StmtType
	paramOIDs   []uint32
	columnOIDs  []uint32
	columnNames []string
	tuples      bool // true if this statement returns rows (SELECT/EXPLAIN/CALL/PRAGMA/...)
}

func (p *preparedStatement) close() {
	if p.stmt != nil {
		p.stmt.Close()
		p.stmt = nil
	}
}

// prepareStatement prepares query against dconn and gathers the metadata
// needed to answer Describe (parameter/column OIDs) without executing it.
// clientParamOIDs comes from a Parse message's ParameterOIDs and may be
// nil/short -- any parameter the client didn't pin a type for is resolved
// from what DuckDB inferred while parsing the query.
func prepareStatement(ctx context.Context, dconn *duckdb.Conn, name, query string, clientParamOIDs []uint32) (*preparedStatement, error) {
	driverStmt, err := dconn.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	stmt := driverStmt.(*duckdb.Stmt)

	st, err := stmt.StatementType()
	if err != nil {
		stmt.Close()
		return nil, err
	}

	numInput := stmt.NumInput()
	paramOIDs := make([]uint32, numInput)
	for i := 0; i < numInput; i++ {
		if i < len(clientParamOIDs) && clientParamOIDs[i] != 0 {
			paramOIDs[i] = clientParamOIDs[i]
			continue
		}
		pt, err := stmt.ParamType(i + 1)
		if err != nil || pt == duckdb.TYPE_INVALID {
			paramOIDs[i] = pgtype.TextOID
			continue
		}
		paramOIDs[i] = oidForDuckType(pt)
	}

	tuples := isTupleReturning(st)

	var columnOIDs []uint32
	var columnNames []string
	if tuples {
		colCount, err := stmt.ColumnCount()
		if err != nil {
			stmt.Close()
			return nil, err
		}
		columnOIDs = make([]uint32, colCount)
		columnNames = make([]string, colCount)
		for i := 0; i < colCount; i++ {
			ct, err := stmt.ColumnType(i)
			if err != nil {
				stmt.Close()
				return nil, err
			}
			cn, err := stmt.ColumnName(i)
			if err != nil {
				stmt.Close()
				return nil, err
			}
			columnOIDs[i] = oidForDuckType(ct)
			columnNames[i] = cn
		}
	}

	return &preparedStatement{
		name:        name,
		query:       query,
		stmt:        stmt,
		stmtType:    st,
		paramOIDs:   paramOIDs,
		columnOIDs:  columnOIDs,
		columnNames: columnNames,
		tuples:      tuples,
	}, nil
}

func isTupleReturning(st duckdb.StmtType) bool {
	switch st {
	case duckdb.STATEMENT_TYPE_SELECT, duckdb.STATEMENT_TYPE_EXPLAIN, duckdb.STATEMENT_TYPE_CALL, duckdb.STATEMENT_TYPE_PRAGMA:
		return true
	default:
		return false
	}
}

// commandTag builds a Postgres CommandComplete tag for a non-tuple-
// returning statement. DuckDB's StmtType doesn't disambiguate CREATE
// TABLE/INDEX/VIEW etc., so DDL tags are best-effort.
func commandTag(st duckdb.StmtType, rowsAffected int64) string {
	switch st {
	case duckdb.STATEMENT_TYPE_INSERT:
		return fmt.Sprintf("INSERT 0 %d", rowsAffected)
	case duckdb.STATEMENT_TYPE_UPDATE:
		return fmt.Sprintf("UPDATE %d", rowsAffected)
	case duckdb.STATEMENT_TYPE_DELETE:
		return fmt.Sprintf("DELETE %d", rowsAffected)
	case duckdb.STATEMENT_TYPE_CREATE:
		return "CREATE TABLE"
	case duckdb.STATEMENT_TYPE_DROP:
		return "DROP TABLE"
	case duckdb.STATEMENT_TYPE_ALTER:
		return "ALTER TABLE"
	case duckdb.STATEMENT_TYPE_TRANSACTION:
		return "" // overwritten by the BEGIN/COMMIT/ROLLBACK keyword tag in runStatement
	default:
		return "OK"
	}
}

// buildRowDescription builds the RowDescription for stmt's columns. Format
// is always reported as text (0) here, matching real Postgres servers --
// the actual wire format used for results is negotiated later by Bind, not
// anticipated in RowDescription.
func buildRowDescription(stmt *preparedStatement) *pgproto3.RowDescription {
	fields := make([]pgproto3.FieldDescription, len(stmt.columnNames))
	for i := range fields {
		fields[i] = pgproto3.FieldDescription{
			Name:         []byte(stmt.columnNames[i]),
			DataTypeOID:  stmt.columnOIDs[i],
			DataTypeSize: -1,
			Format:       0,
		}
	}
	return &pgproto3.RowDescription{Fields: fields}
}

// portal is created by Bind from a preparedStatement.
type portal struct {
	name          string
	stmt          *preparedStatement
	resultFormats []int16
}

// formatFor resolves the wire format code for column i, per Bind's
// ResultFormatCodes rules (see resolveFormat).
func (p *portal) formatFor(i int) int16 {
	return resolveFormat(p.resultFormats, i)
}
