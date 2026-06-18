package duckpgwire

import (
	"context"
	"database/sql"
	"fmt"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"
)

// preparedStatement is created by a Parse message (or, for the simple
// query protocol, synthesized for the single statement in a Query
// message). Unlike before, there is no live low-level statement handle to
// hold -- duckproxy never caches prepared statements either, so the
// statement gets re-described here and executed-with-its-args-included
// later via an ordinary database/sql call.
type preparedStatement struct {
	name        string
	query       string
	stmtType    duckdb.StmtType
	paramOIDs   []uint32
	columnOIDs  []uint32
	columnNames []string
	tuples      bool // true if this statement returns rows (SELECT/EXPLAIN/CALL/PRAGMA/...)
}

// prepareStatement describes query against duckproxy and gathers the
// metadata needed to answer Describe (parameter/column OIDs) without
// executing it. clientParamOIDs comes from a Parse message's
// ParameterOIDs and may be nil/short -- any parameter the client didn't
// pin a type for is resolved from what DuckDB inferred while parsing the
// query. sqlConn is always the session's original *sql.Conn, even while a
// transaction is open -- duckproxy.Describe uses (*sql.Conn).Raw, which
// *sql.Tx doesn't have, but it's a read-only, non-executing operation on
// the same underlying connection a *sql.Tx on it would use, so this is
// safe regardless of transaction state.
func prepareStatement(ctx context.Context, sqlConn *sql.Conn, name, query string, clientParamOIDs []uint32) (*preparedStatement, error) {
	described, err := duckproxy.Describe(ctx, sqlConn, query)
	if err != nil {
		return nil, err
	}

	paramOIDs := make([]uint32, len(described.Params))
	for i, p := range described.Params {
		if i < len(clientParamOIDs) && clientParamOIDs[i] != 0 {
			paramOIDs[i] = clientParamOIDs[i]
			continue
		}
		if p.Type == duckdb.TYPE_INVALID {
			paramOIDs[i] = pgtype.TextOID
			continue
		}
		paramOIDs[i] = oidForDuckType(p.Type)
	}

	var columnOIDs []uint32
	var columnNames []string
	if described.Tuples {
		columnOIDs = make([]uint32, len(described.Columns))
		columnNames = make([]string, len(described.Columns))
		for i, c := range described.Columns {
			columnOIDs[i] = oidForDuckType(c.Type)
			columnNames[i] = c.Name
		}
	}

	return &preparedStatement{
		name:        name,
		query:       query,
		stmtType:    described.StatementType,
		paramOIDs:   paramOIDs,
		columnOIDs:  columnOIDs,
		columnNames: columnNames,
		tuples:      described.Tuples,
	}, nil
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

// portal is created by Bind from a preparedStatement, holding the
// decoded argument values Bind received -- there's no live prepared
// statement to have bound them into ahead of time.
type portal struct {
	name          string
	stmt          *preparedStatement
	args          []any
	resultFormats []int16
}

// formatFor resolves the wire format code for column i, per Bind's
// ResultFormatCodes rules (see resolveFormat).
func (p *portal) formatFor(i int) int16 {
	return resolveFormat(p.resultFormats, i)
}
