package duckpgwire

import (
	"database/sql/driver"
	"fmt"
	"time"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

// encodeValue renders v, a value produced by duckdb's driver.Rows.Next, as
// the wire bytes for a DataRow field of the given Postgres oid and format
// code. A nil, nil return means SQL NULL.
func encodeValue(oid uint32, format int16, v driver.Value) ([]byte, error) {
	if v == nil {
		return nil, nil
	}

	switch oid {
	case pgtype.TextOID:
		// Catches DuckDB types with no clean Postgres equivalent
		// (Decimal, *big.Int, BIT, LIST/STRUCT/MAP/ARRAY/UNION, ...) as
		// well as VARCHAR/ENUM. pgtype's text codec only accepts
		// string/[]byte/TextValuer, so stringify everything else first.
		return []byte(stringify(v)), nil
	case pgtype.UUIDOID:
		if b, ok := v.([]byte); ok && len(b) == 16 {
			var u pgtype.UUID
			copy(u.Bytes[:], b)
			u.Valid = true
			v = u
		}
	case pgtype.IntervalOID:
		if iv, ok := v.(duckdb.Interval); ok {
			v = pgtype.Interval{Microseconds: iv.Micros, Days: iv.Days, Months: iv.Months, Valid: true}
		}
	case pgtype.DateOID:
		if t, ok := v.(time.Time); ok {
			v = pgtype.Date{Time: t, Valid: true}
		}
	case pgtype.TimestampOID:
		if t, ok := v.(time.Time); ok {
			v = pgtype.Timestamp{Time: t, Valid: true}
		}
	case pgtype.TimestamptzOID:
		if t, ok := v.(time.Time); ok {
			v = pgtype.Timestamptz{Time: t, Valid: true}
		}
	case pgtype.TimeOID:
		if t, ok := v.(time.Time); ok {
			midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
			v = pgtype.Time{Microseconds: t.Sub(midnight).Microseconds(), Valid: true}
		}
	}

	switch x := v.(type) {
	case int8:
		v = int64(x)
	case int16:
		v = int64(x)
	case int32:
		v = int64(x)
	case uint8:
		v = int64(x)
	case uint16:
		v = int64(x)
	case uint32:
		v = int64(x)
	case float32:
		v = float64(x)
	}

	return pgTypeMap.Encode(oid, format, v, nil)
}

func stringify(v any) string {
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("%v", v)
}
