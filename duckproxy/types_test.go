package duckproxy

import (
	"testing"
	"time"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestOidForDuckType(t *testing.T) {
	cases := []struct {
		in   duckdb.Type
		want uint32
	}{
		{duckdb.TYPE_BOOLEAN, pgtype.BoolOID},
		{duckdb.TYPE_TINYINT, pgtype.Int2OID},
		{duckdb.TYPE_SMALLINT, pgtype.Int2OID},
		{duckdb.TYPE_INTEGER, pgtype.Int4OID},
		{duckdb.TYPE_BIGINT, pgtype.Int8OID},
		{duckdb.TYPE_FLOAT, pgtype.Float4OID},
		{duckdb.TYPE_DOUBLE, pgtype.Float8OID},
		{duckdb.TYPE_VARCHAR, pgtype.TextOID},
		{duckdb.TYPE_ENUM, pgtype.TextOID},
		{duckdb.TYPE_BLOB, pgtype.ByteaOID},
		{duckdb.TYPE_DATE, pgtype.DateOID},
		{duckdb.TYPE_TIME, pgtype.TimeOID},
		{duckdb.TYPE_TIME_TZ, pgtype.TextOID}, // pgtype has no timetz codec
		{duckdb.TYPE_TIMESTAMP, pgtype.TimestampOID},
		{duckdb.TYPE_TIMESTAMP_TZ, pgtype.TimestamptzOID},
		{duckdb.TYPE_INTERVAL, pgtype.IntervalOID},
		{duckdb.TYPE_UUID, pgtype.UUIDOID},
		{duckdb.TYPE_UBIGINT, pgtype.TextOID},
		{duckdb.TYPE_HUGEINT, pgtype.TextOID},
		{duckdb.TYPE_DECIMAL, pgtype.TextOID},
		{duckdb.TYPE_LIST, pgtype.TextOID},
		{duckdb.TYPE_STRUCT, pgtype.TextOID},
		{duckdb.TYPE_BIT, pgtype.TextOID},
	}

	for _, c := range cases {
		if got := oidForDuckType(c.in); got != c.want {
			t.Errorf("oidForDuckType(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	ts := time.Date(2024, 3, 15, 9, 30, 0, 0, time.UTC)

	cases := []struct {
		name string
		oid  uint32
		in   any
		want any
	}{
		{"bool", pgtype.BoolOID, true, true},
		{"int2", pgtype.Int2OID, int16(7), int64(7)},
		{"int4", pgtype.Int4OID, int32(42), int64(42)},
		{"int8", pgtype.Int8OID, int64(123456789), int64(123456789)},
		{"float4", pgtype.Float4OID, float32(1.5), float64(1.5)},
		{"float8", pgtype.Float8OID, float64(2.25), float64(2.25)},
		{"text", pgtype.TextOID, "hello", "hello"},
		{"bytea", pgtype.ByteaOID, []byte{1, 2, 3}, []byte{1, 2, 3}},
		{"timestamp", pgtype.TimestampOID, ts, ts},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wire, err := encodeValue(c.oid, pgtype.TextFormatCode, c.in)
			if err != nil {
				t.Fatalf("encodeValue: %v", err)
			}

			got, err := decodeParam(c.oid, pgtype.TextFormatCode, wire)
			if err != nil {
				t.Fatalf("decodeParam: %v", err)
			}

			switch want := c.want.(type) {
			case time.Time:
				gotTime, ok := got.(time.Time)
				if !ok || !gotTime.Equal(want) {
					t.Errorf("got %#v, want %#v", got, want)
				}
			case []byte:
				gotBytes, ok := got.([]byte)
				if !ok || string(gotBytes) != string(want) {
					t.Errorf("got %#v, want %#v", got, want)
				}
			default:
				if got != c.want {
					t.Errorf("got %#v (%T), want %#v (%T)", got, got, c.want, c.want)
				}
			}
		})
	}
}

func TestEncodeValueNull(t *testing.T) {
	b, err := encodeValue(pgtype.TextOID, pgtype.TextFormatCode, nil)
	if err != nil {
		t.Fatalf("encodeValue: %v", err)
	}
	if b != nil {
		t.Errorf("expected nil for NULL, got %v", b)
	}
}

func TestDecodeParamNull(t *testing.T) {
	v, err := decodeParam(pgtype.TextOID, pgtype.TextFormatCode, nil)
	if err != nil {
		t.Fatalf("decodeParam: %v", err)
	}
	if v != nil {
		t.Errorf("expected nil for NULL, got %v", v)
	}
}

func TestFormatUUID(t *testing.T) {
	var b [16]byte
	for i := range b {
		b[i] = byte(i)
	}
	got := formatUUID(b)
	want := "00010203-0405-0607-0809-0a0b0c0d0e0f"
	if got != want {
		t.Errorf("formatUUID() = %q, want %q", got, want)
	}
}
