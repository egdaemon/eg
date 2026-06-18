package duckpgwire

import (
	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

// pgTypeMap is used only as a value<->wire-bytes codec for the handful of
// scalar Postgres types below. It never drives SQL-level type decisions --
// once a parameter is decoded into a Go-native value (bool, int64, float64,
// string, []byte, time.Time), DuckDB's own Bind/Stmt machinery is what
// actually interprets and casts it.
var pgTypeMap = pgtype.NewMap()

// oidForDuckType maps a DuckDB column type to the closest Postgres OID, for
// RowDescription/ParameterDescription. DuckDB types with no clean Postgres
// equivalent (unsigned ints wider than int4, HUGEINT/UHUGEINT/BIGNUM/
// DECIMAL, ENUM, BIT, TIME_TZ (pgtype has no timetz codec), LIST/STRUCT/
// MAP/ARRAY/UNION) fall back to text -- they
// remain valid SQL values, just flagged to the client as text rather than a
// type-specific encoding.
func oidForDuckType(t duckdb.Type) uint32 {
	switch t {
	case duckdb.TYPE_BOOLEAN:
		return pgtype.BoolOID
	case duckdb.TYPE_TINYINT, duckdb.TYPE_SMALLINT, duckdb.TYPE_UTINYINT:
		return pgtype.Int2OID
	case duckdb.TYPE_INTEGER, duckdb.TYPE_USMALLINT:
		return pgtype.Int4OID
	case duckdb.TYPE_BIGINT, duckdb.TYPE_UINTEGER:
		return pgtype.Int8OID
	case duckdb.TYPE_FLOAT:
		return pgtype.Float4OID
	case duckdb.TYPE_DOUBLE:
		return pgtype.Float8OID
	case duckdb.TYPE_DATE:
		return pgtype.DateOID
	case duckdb.TYPE_TIME:
		return pgtype.TimeOID
	case duckdb.TYPE_TIMESTAMP, duckdb.TYPE_TIMESTAMP_S, duckdb.TYPE_TIMESTAMP_MS, duckdb.TYPE_TIMESTAMP_NS:
		return pgtype.TimestampOID
	case duckdb.TYPE_TIMESTAMP_TZ:
		return pgtype.TimestamptzOID
	case duckdb.TYPE_INTERVAL:
		return pgtype.IntervalOID
	case duckdb.TYPE_UUID:
		return pgtype.UUIDOID
	case duckdb.TYPE_BLOB, duckdb.TYPE_GEOMETRY:
		return pgtype.ByteaOID
	default:
		return pgtype.TextOID
	}
}
