package duckpgwire

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestValueRoundtrip(t *testing.T) {
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
			require.NoError(t, err)

			got, err := decodeParam(c.oid, pgtype.TextFormatCode, wire)
			require.NoError(t, err)

			switch want := c.want.(type) {
			case time.Time:
				gotTime, ok := got.(time.Time)
				require.True(t, ok)
				require.True(t, gotTime.Equal(want))
			default:
				require.Equal(t, c.want, got)
			}
		})
	}

	t.Run("encode_null", func(t *testing.T) {
		b, err := encodeValue(pgtype.TextOID, pgtype.TextFormatCode, nil)
		require.NoError(t, err)
		require.Nil(t, b)
	})

	t.Run("decode_null", func(t *testing.T) {
		v, err := decodeParam(pgtype.TextOID, pgtype.TextFormatCode, nil)
		require.NoError(t, err)
		require.Nil(t, v)
	})
}
