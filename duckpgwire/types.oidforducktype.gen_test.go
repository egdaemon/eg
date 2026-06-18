package duckpgwire

import (
	"testing"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestTypesOidForDuckType(t *testing.T) {
	cases := []struct {
		name string
		in   duckdb.Type
		want uint32
	}{
		{"boolean", duckdb.TYPE_BOOLEAN, pgtype.BoolOID},
		{"tinyint", duckdb.TYPE_TINYINT, pgtype.Int2OID},
		{"smallint", duckdb.TYPE_SMALLINT, pgtype.Int2OID},
		{"integer", duckdb.TYPE_INTEGER, pgtype.Int4OID},
		{"bigint", duckdb.TYPE_BIGINT, pgtype.Int8OID},
		{"float", duckdb.TYPE_FLOAT, pgtype.Float4OID},
		{"double", duckdb.TYPE_DOUBLE, pgtype.Float8OID},
		{"varchar", duckdb.TYPE_VARCHAR, pgtype.TextOID},
		{"enum", duckdb.TYPE_ENUM, pgtype.TextOID},
		{"blob", duckdb.TYPE_BLOB, pgtype.ByteaOID},
		{"date", duckdb.TYPE_DATE, pgtype.DateOID},
		{"time", duckdb.TYPE_TIME, pgtype.TimeOID},
		{"time_tz_falls_back_to_text", duckdb.TYPE_TIME_TZ, pgtype.TextOID}, // pgtype has no timetz codec
		{"timestamp", duckdb.TYPE_TIMESTAMP, pgtype.TimestampOID},
		{"timestamptz", duckdb.TYPE_TIMESTAMP_TZ, pgtype.TimestamptzOID},
		{"interval", duckdb.TYPE_INTERVAL, pgtype.IntervalOID},
		{"uuid", duckdb.TYPE_UUID, pgtype.UUIDOID},
		{"ubigint_falls_back_to_text", duckdb.TYPE_UBIGINT, pgtype.TextOID},
		{"hugeint_falls_back_to_text", duckdb.TYPE_HUGEINT, pgtype.TextOID},
		{"decimal_falls_back_to_text", duckdb.TYPE_DECIMAL, pgtype.TextOID},
		{"list_falls_back_to_text", duckdb.TYPE_LIST, pgtype.TextOID},
		{"struct_falls_back_to_text", duckdb.TYPE_STRUCT, pgtype.TextOID},
		{"bit_falls_back_to_text", duckdb.TYPE_BIT, pgtype.TextOID},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, oidForDuckType(c.in))
		})
	}
}
