package duckproxy

import (
	"database/sql/driver"
	"encoding/hex"
	"time"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

// decodeParam turns the wire bytes of a single Bind parameter into the
// Go-native value DuckDB's own Bind/Stmt machinery expects (bool, int64,
// float64, string, []byte, time.Time, duckdb.Interval) for the resolved
// Postgres oid. DuckDB itself -- not this function -- is responsible for
// any further type coercion (e.g. casting a string into a UUID or BIT
// column); decodeParam only bridges wire bytes to a Go value.
//
// raw == nil means SQL NULL.
func decodeParam(oid uint32, format int16, raw []byte) (driver.Value, error) {
	if raw == nil {
		return nil, nil
	}

	switch oid {
	case pgtype.BoolOID:
		var v bool
		if err := pgTypeMap.Scan(oid, format, raw, &v); err != nil {
			return nil, err
		}
		return v, nil

	case pgtype.Int2OID, pgtype.Int4OID, pgtype.Int8OID:
		var v int64
		if err := pgTypeMap.Scan(oid, format, raw, &v); err != nil {
			return nil, err
		}
		return v, nil

	case pgtype.Float4OID, pgtype.Float8OID:
		var v float64
		if err := pgTypeMap.Scan(oid, format, raw, &v); err != nil {
			return nil, err
		}
		return v, nil

	case pgtype.ByteaOID:
		var v []byte
		if err := pgTypeMap.Scan(oid, format, raw, &v); err != nil {
			return nil, err
		}
		return v, nil

	case pgtype.DateOID:
		var v pgtype.Date
		if err := pgTypeMap.Scan(oid, format, raw, &v); err != nil {
			return nil, err
		}
		return v.Time, nil

	case pgtype.TimeOID:
		var v pgtype.Time
		if err := pgTypeMap.Scan(oid, format, raw, &v); err != nil {
			return nil, err
		}
		return time.UnixMicro(v.Microseconds).UTC(), nil

	case pgtype.TimestampOID:
		var v pgtype.Timestamp
		if err := pgTypeMap.Scan(oid, format, raw, &v); err != nil {
			return nil, err
		}
		return v.Time, nil

	case pgtype.TimestamptzOID:
		var v pgtype.Timestamptz
		if err := pgTypeMap.Scan(oid, format, raw, &v); err != nil {
			return nil, err
		}
		return v.Time, nil

	case pgtype.IntervalOID:
		var v pgtype.Interval
		if err := pgTypeMap.Scan(oid, format, raw, &v); err != nil {
			return nil, err
		}
		return duckdb.Interval{Days: v.Days, Months: v.Months, Micros: v.Microseconds}, nil

	case pgtype.UUIDOID:
		var v pgtype.UUID
		if err := pgTypeMap.Scan(oid, format, raw, &v); err != nil {
			return nil, err
		}
		return formatUUID(v.Bytes), nil

	default:
		// TextOID and every DuckDB type with no dedicated Postgres OID
		// (VARCHAR/ENUM/BIT, our fallback bucket, ...): pass the literal
		// bytes through as a Go string and let DuckDB's own implicit
		// casts interpret it -- this is exactly what duckdb-go itself
		// does internally for plain string binds. Clients send these
		// params in text format in practice, since pgx/lib/pq default
		// to text for any OID without a well-known binary codec.
		return string(raw), nil
	}
}

func formatUUID(b [16]byte) string {
	var buf [36]byte
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], b[10:16])
	return string(buf[:])
}
