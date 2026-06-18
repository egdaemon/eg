package drivers

import (
	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/internal/errorsx"
)

// implements the pgx driver https://github.com/jackc/pgx
func init() {
	errorsx.MaybePanic(genieql.RegisterDriver(PGX, NewDriver("github.com/jackc/pgtype", pgx...)))
}

// PGX - driver for github.com/jackc/pgx
const PGX = "github.com/jackc/pgx"

const pgxDefaultDecode = `func() {
	if err := {{ .From | expr }}.AssignTo({{.To | autoreference | expr}}); err != nil {
		return err
	}
}`

const pgxDefaultEncode = `func() {
	if err := {{ .To | expr }}.Set({{ .From | localident | expr }}); err != nil {
		{{ error "err" | ast }}
	}
}`

// https://stackoverflow.com/questions/25065055/what-is-the-maximum-time-time-in-go
const pgxTimeDecode = `func() {
	switch {{ .From | expr }}.InfinityModifier {
	case pgtype.Infinity:
		tmp := time.Unix(math.MaxInt64-62135596800, 999999999)
		{{ .To | autodereference | expr }} = {{ if .Column.Definition.Nullable }}&tmp{{ else }}tmp{{ end }}
	case pgtype.NegativeInfinity:
		tmp := time.Unix(math.MinInt64, math.MinInt64)
		{{ .To | autodereference | expr }} = {{ if .Column.Definition.Nullable }}&tmp{{ else }}tmp{{ end }}
	default:
		if err := {{ .From | expr }}.AssignTo({{ .To | autoreference | expr }}); err != nil {
			return err
		}
	}
}`

const pgxTimeEncode = `func() {
	switch {{ if .Column.Definition.Nullable }}*{{ end }}{{ .From | localident | expr }} {
	case time.Unix(math.MaxInt64-62135596800, 999999999):
		if err := {{ .To | expr }}.Set(pgtype.Infinity); err != nil {
			{{ error "err" | ast }}
		}
	case time.Unix(math.MinInt64, math.MinInt64):
		if err := {{ .To | expr }}.Set(pgtype.NegativeInfinity); err != nil {
			{{ error "err" | ast }}
		}
	default:
		if err := {{ .To | expr }}.Set({{ .From | localident | expr }}); err != nil {
			{{ error "err" | ast }}
		}
	}
}`

var pgx = []genieql.ColumnDefinition{
	{
		Type:       "pgtype.OID",
		Native:     stringExprString,
		ColumnType: "pgtype.OID",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.OIDValue",
		Native:     stringExprString,
		ColumnType: "pgtype.OIDValue",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.CIDR",
		Native:     cidrExpr,
		ColumnType: "pgtype.CIDR",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.CIDRArray",
		Native:     cidrArrayExpr,
		ColumnType: "pgtype.CIDRArray",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Macaddr",
		Native:     macExpr,
		ColumnType: "pgtype.Macaddr",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Name",
		Native:     stringExprString,
		ColumnType: "pgtype.Name",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Inet",
		Native:     ipExpr,
		ColumnType: "pgtype.Inet",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Numeric",
		Native:     float64ExprString,
		ColumnType: "pgtype.Numeric",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Bytea",
		Native:     bytesExpr,
		ColumnType: "pgtype.Bytea",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Bit",
		Native:     bytesExpr,
		ColumnType: "pgtype.Bit",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Varbit",
		Native:     bytesExpr,
		ColumnType: "pgtype.Varbit",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Bool",
		Native:     boolExprString,
		ColumnType: "pgtype.Bool",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Float4",
		Native:     float32ExprString,
		ColumnType: "pgtype.Float4",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Float8",
		Native:     float64ExprString,
		ColumnType: "pgtype.Float8",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Int2",
		Native:     intExprString,
		ColumnType: "pgtype.Int2",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Int2Array",
		Native:     intArrExpr,
		ColumnType: "pgtype.Int2Array",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Int4",
		Native:     intExprString,
		ColumnType: "pgtype.Int4",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Int4Array",
		Native:     intArrExpr,
		ColumnType: "pgtype.Int4Array",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Int8",
		Native:     intExprString,
		ColumnType: "pgtype.Int8",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Int8Array",
		Native:     intArrExpr,
		ColumnType: "pgtype.Int8Array",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Text",
		Native:     stringExprString,
		ColumnType: "pgtype.Text",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.TextArray",
		Native:     stringArrExpr,
		ColumnType: "pgtype.TextArray",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Varchar",
		Native:     stringExprString,
		ColumnType: "pgtype.Varchar",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.BPChar",
		Native:     stringExprString,
		ColumnType: "pgtype.BPChar",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Date",
		Native:     timeExprString,
		ColumnType: "pgtype.Date",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.Timestamp",
		Native:     timeExprString,
		ColumnType: "pgtype.Timestamp",
		Decode:     pgxTimeDecode,
		Encode:     pgxTimeEncode,
	},
	{
		Type:       "pgtype.Timestamptz",
		Native:     timeExprString,
		ColumnType: "pgtype.Timestamptz",
		Decode:     pgxTimeDecode,
		Encode:     pgxTimeEncode,
	},
	{
		Type:       "pgtype.Interval",
		Native:     durationExpr,
		ColumnType: "pgtype.Interval",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.UUID",
		Native:     stringExprString,
		ColumnType: "pgtype.UUID",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.UUIDArray",
		Native:     stringArrExpr,
		ColumnType: "pgtype.UUIDArray",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.JSONB",
		Native:     bytesExpr,
		ColumnType: "pgtype.JSONB",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "pgtype.JSON",
		Native:     bytesExpr,
		ColumnType: "pgtype.JSON",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "json.RawMessage",
		Native:     bytesExpr,
		ColumnType: "pgtype.JSON",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*json.RawMessage",
		Nullable:   true,
		Native:     bytesExpr,
		ColumnType: "pgtype.JSON",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "net.IPNet",
		Native:     cidrExpr,
		ColumnType: "pgtype.CIDR",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*net.IPNet",
		Nullable:   true,
		Native:     cidrExpr,
		ColumnType: "pgtype.CIDR",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "[]net.IPNet",
		Native:     cidrArrayExpr,
		ColumnType: "pgtype.CIDRArray",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*[]net.IPNet",
		Nullable:   true,
		Native:     cidrArrayExpr,
		ColumnType: "pgtype.CIDRArray",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "net.IP",
		Native:     ipExpr,
		ColumnType: "pgtype.Inet",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*net.IP",
		Nullable:   true,
		Native:     ipExpr,
		ColumnType: "pgtype.Inet",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "[]byte",
		Native:     bytesExpr,
		ColumnType: "pgtype.Bytea",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*[]byte",
		Native:     bytesExpr,
		ColumnType: "pgtype.Bytea",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "[]string",
		Native:     stringArrExpr,
		ColumnType: "pgtype.TextArray",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*[]string",
		Native:     stringArrExpr,
		ColumnType: "pgtype.TextArray",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "[]int",
		Native:     intArrExpr,
		ColumnType: "pgtype.Int8Array",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*[]int",
		Native:     intArrExpr,
		ColumnType: "pgtype.Int8Array",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "time.Duration",
		Native:     durationExpr,
		ColumnType: "pgtype.Interval",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*time.Duration",
		Native:     durationExpr,
		ColumnType: "pgtype.Interval",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "net.HardwareAddr",
		Native:     macExpr,
		ColumnType: "pgtype.Macaddr",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*net.HardwareAddr",
		Native:     macExpr,
		ColumnType: "pgtype.Macaddr",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "float32",
		Native:     float32ExprString,
		ColumnType: "pgtype.Float4",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*float32",
		Native:     float32ExprString,
		ColumnType: "pgtype.Float4",
		Nullable:   true,
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "float64",
		Native:     float64ExprString,
		ColumnType: "pgtype.Float8",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*float64",
		Native:     float64ExprString,
		ColumnType: "pgtype.Float8",
		Nullable:   true,
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "string",
		Native:     stringExprString,
		ColumnType: "pgtype.Text",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*string",
		Nullable:   true,
		Native:     stringExprString,
		ColumnType: "pgtype.Text",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "int16",
		Native:     intExprString,
		ColumnType: "pgtype.Int2",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*int16",
		Native:     intExprString,
		ColumnType: "pgtype.Int2",
		Nullable:   true,
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "int32",
		Native:     intExprString,
		ColumnType: "pgtype.Int4",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*int32",
		Native:     intExprString,
		ColumnType: "pgtype.Int4",
		Nullable:   true,
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "int64",
		Native:     intExprString,
		ColumnType: "pgtype.Int8",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*int64",
		Native:     intExprString,
		ColumnType: "pgtype.Int8",
		Nullable:   true,
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "int",
		Native:     intExprString,
		ColumnType: "pgtype.Int8",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*int",
		Native:     intExprString,
		ColumnType: "pgtype.Int8",
		Nullable:   true,
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "time.Time",
		Native:     timeExprString,
		ColumnType: "pgtype.Timestamptz",
		Decode:     pgxTimeDecode,
		Encode:     pgxTimeEncode,
	},
	{
		Type:       "*time.Time",
		Native:     timeExprString,
		ColumnType: "pgtype.Timestamptz",
		Nullable:   true,
		Decode:     pgxTimeDecode,
		Encode:     pgxTimeEncode,
	},
	{
		Type:       "bool",
		Native:     boolExprString,
		ColumnType: "pgtype.Bool",
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
	{
		Type:       "*bool",
		Native:     boolExprString,
		ColumnType: "pgtype.Bool",
		Nullable:   true,
		Decode:     pgxDefaultDecode,
		Encode:     pgxDefaultEncode,
	},
}
