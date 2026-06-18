package duckproxyserver

import (
	"database/sql/driver"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/duckproxy"
)

// toProtoValue extends duckproxy.ToProtoValue with the two DuckDB-native
// types that only ever appear on this (real-duckdb-backed) side of the
// protocol -- duckdb.Interval and duckdb.Decimal, produced directly by
// duckdb-go's driver.Rows.Next. Kept here, not in duckproxy, so duckproxy
// itself never needs to import duckdb-go's cgo bindings.
func toProtoValue(v driver.Value) (*duckproxy.Value, error) {
	switch x := v.(type) {
	case duckdb.Interval:
		return &duckproxy.Value{Kind: &duckproxy.Value_IntervalValue{IntervalValue: &duckproxy.Interval{
			Months: x.Months,
			Days:   x.Days,
			Micros: x.Micros,
		}}}, nil
	case duckdb.Decimal:
		return &duckproxy.Value{Kind: &duckproxy.Value_DecimalValue{DecimalValue: x.String()}}, nil
	default:
		return duckproxy.ToProtoValue(v)
	}
}

// fromProtoValue extends duckproxy.FromProtoValue by converting an
// interval/decimal back into duckdb-go's own types, since values bound as
// query parameters here go straight into a real duckdb-go statement, which
// only recognizes its own Interval/Decimal -- not duckproxy's wire structs.
func fromProtoValue(v *duckproxy.Value) (driver.Value, error) {
	gv, err := duckproxy.FromProtoValue(v)
	if err != nil {
		return nil, err
	}

	switch x := gv.(type) {
	case *duckproxy.Interval:
		return duckdb.Interval{Months: x.Months, Days: x.Days, Micros: x.Micros}, nil
	default:
		return gv, nil
	}
}
