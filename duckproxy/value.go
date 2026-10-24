package duckproxy

import (
	"database/sql/driver"
	"fmt"
	"math"
	"math/big"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ToProtoValue converts a Go value -- either a query parameter already
// normalized by database/sql's default converter, or a column value
// produced directly by duckdb-go's driver.Rows.Next -- into the wire
// representation. Unlike a Postgres OID table, this only has to describe
// DuckDB's own native types. duckdb.Interval/duckdb.Decimal aren't handled
// here directly -- only the server side (duckproxyserver) ever actually
// produces those from a real DuckDB row, and importing duckdb-go at all
// pulls in its cgo bindings, which this package must stay free of so it can
// build for GOOS=wasip1 (the wasm guest is a client of this package).
func ToProtoValue(v driver.Value) (*Value, error) {
	if v == nil {
		return &Value{Kind: &Value_IsNull{IsNull: true}}, nil
	}

	switch x := v.(type) {
	case bool:
		return &Value{Kind: &Value_BoolValue{BoolValue: x}}, nil
	case int8:
		return &Value{Kind: &Value_IntValue{IntValue: int64(x)}}, nil
	case int16:
		return &Value{Kind: &Value_IntValue{IntValue: int64(x)}}, nil
	case int32:
		return &Value{Kind: &Value_IntValue{IntValue: int64(x)}}, nil
	case int64:
		return &Value{Kind: &Value_IntValue{IntValue: x}}, nil
	case int:
		return &Value{Kind: &Value_IntValue{IntValue: int64(x)}}, nil
	case uint8:
		return &Value{Kind: &Value_IntValue{IntValue: int64(x)}}, nil
	case uint16:
		return &Value{Kind: &Value_IntValue{IntValue: int64(x)}}, nil
	case uint32:
		return &Value{Kind: &Value_IntValue{IntValue: int64(x)}}, nil
	case uint64:
		// UBIGINT can exceed math.MaxInt64; only widen to IntValue when it
		// fits, otherwise fall back to the bignum string form.
		if x <= math.MaxInt64 {
			return &Value{Kind: &Value_IntValue{IntValue: int64(x)}}, nil
		}
		return &Value{Kind: &Value_BignumValue{BignumValue: new(big.Int).SetUint64(x).String()}}, nil
	case float32:
		return &Value{Kind: &Value_DoubleValue{DoubleValue: float64(x)}}, nil
	case float64:
		return &Value{Kind: &Value_DoubleValue{DoubleValue: x}}, nil
	case string:
		return &Value{Kind: &Value_StringValue{StringValue: x}}, nil
	case []byte:
		return &Value{Kind: &Value_BytesValue{BytesValue: x}}, nil
	case time.Time:
		return &Value{Kind: &Value_TimestampValue{TimestampValue: timestamppb.New(x)}}, nil
	case Interval:
		return &Value{Kind: &Value_IntervalValue{IntervalValue: &x}}, nil
	case *Interval:
		return &Value{Kind: &Value_IntervalValue{IntervalValue: x}}, nil
	case *big.Int:
		return &Value{Kind: &Value_BignumValue{BignumValue: x.String()}}, nil
	case []any:
		values := make([]*Value, len(x))
		for i, elem := range x {
			ev, err := ToProtoValue(elem)
			if err != nil {
				return nil, err
			}
			values[i] = ev
		}
		return &Value{Kind: &Value_ListValue{ListValue: &ListValue{Values: values}}}, nil
	case map[string]any:
		fields := make(map[string]*Value, len(x))
		for k, elem := range x {
			ev, err := ToProtoValue(elem)
			if err != nil {
				return nil, err
			}
			fields[k] = ev
		}
		return &Value{Kind: &Value_StructValue{StructValue: &StructValue{Fields: fields}}}, nil
	default:
		return nil, fmt.Errorf("duckproxy: unsupported value type %T", v)
	}
}

// FromProtoValue is the inverse of ToProtoValue: it turns a wire Value back
// into the Go-native value duckdb-go's own Bind/Stmt machinery (server
// side) or a database/sql caller's Scan (client side) expects. An interval
// decodes to the wire Interval struct directly (Months/Days/Micros) rather
// than duckdb.Interval, for the same cgo-free reason as ToProtoValue; a
// decimal decodes to its already-formatted string.
func FromProtoValue(v *Value) (driver.Value, error) {
	if v == nil {
		return nil, nil
	}

	switch k := v.GetKind().(type) {
	case *Value_IsNull:
		return nil, nil
	case *Value_BoolValue:
		return k.BoolValue, nil
	case *Value_IntValue:
		return k.IntValue, nil
	case *Value_DoubleValue:
		return k.DoubleValue, nil
	case *Value_StringValue:
		return k.StringValue, nil
	case *Value_BytesValue:
		return k.BytesValue, nil
	case *Value_TimestampValue:
		return k.TimestampValue.AsTime(), nil
	case *Value_IntervalValue:
		return &Interval{
			Months: k.IntervalValue.GetMonths(),
			Days:   k.IntervalValue.GetDays(),
			Micros: k.IntervalValue.GetMicros(),
		}, nil
	case *Value_DecimalValue:
		return k.DecimalValue, nil
	case *Value_BignumValue:
		bi, ok := new(big.Int).SetString(k.BignumValue, 10)
		if !ok {
			return nil, fmt.Errorf("duckproxy: invalid bignum value %q", k.BignumValue)
		}
		return bi, nil
	case *Value_ListValue:
		out := make([]any, len(k.ListValue.GetValues()))
		for i, elem := range k.ListValue.GetValues() {
			ev, err := FromProtoValue(elem)
			if err != nil {
				return nil, err
			}
			out[i] = ev
		}
		return out, nil
	case *Value_StructValue:
		fields := k.StructValue.GetFields()
		out := make(map[string]any, len(fields))
		for key, elem := range fields {
			ev, err := FromProtoValue(elem)
			if err != nil {
				return nil, err
			}
			out[key] = ev
		}
		return out, nil
	case nil:
		return nil, fmt.Errorf("duckproxy: empty Value")
	default:
		return nil, fmt.Errorf("duckproxy: unknown Value kind %T", k)
	}
}
