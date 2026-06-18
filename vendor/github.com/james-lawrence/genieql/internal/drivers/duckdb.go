package drivers

import (
	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/internal/errorsx"
)

const DuckDB = "github.com/marcboeker/go-duckdb"

// implements the duckdb driver https://github.com/marcboeker/go-duckdb
func init() {
	errorsx.MaybePanic(genieql.RegisterDriver(DuckDB, NewDriver(DuckDB, ddb...)))
}

const (
	ddbDecodeUUID = `func() {
		if {{ .From | expr }}.Valid {
			if uid, err := uuid.FromBytes([]byte({{ .From | expr }}.String)); err != nil {
				return err
			} else {
				{{ .To | autodereference | expr }} = uid.String()
			}
		}
	}`

	ddbEncodeTime = `func() {
		switch ts := {{ if .Column.Definition.Nullable }}*{{ end }}{{ .From | localident | expr }}; {
		case time.Unix(math.MaxInt64-62135596800, 999999999).Equal(ts):
			{{ .To | expr }}.Infinity()
		case time.Unix(math.MinInt64, math.MinInt64).Equal(ts):
			{{ .To | expr }}.NegativeInfinity()
		default:
			{{ .To | expr }}.Status = ducktype.Present
			{{ .To | expr }}.Time = {{ .From | localident | expr }}
		}
	}`

	ddbDecodeTime = `func() {
		switch {{ .From | expr }}.InfinityModifier {
		case ducktype.Infinity:
			tmp := time.Unix(math.MaxInt64-62135596800, 999999999)
			{{ .To | autodereference | expr }} = {{ if .Column.Definition.Nullable }}&tmp{{ else }}tmp{{ end }}
		case ducktype.NegativeInfinity:
			tmp := time.Unix(math.MinInt64, math.MinInt64)
			{{ .To | autodereference | expr }} = {{ if .Column.Definition.Nullable }}&tmp{{ else }}tmp{{ end }}
		default:
			{{ .To | autodereference | expr }} = {{ .From | localident | expr }}.Time
		}
	}`

	ddbDecodeBinary = `func() {
		{{ .To | expr }} ={{ .From | expr }}
	}`

	ddbEncodeBinary = `func() {
		{{ .To | expr }} = {{ .From | expr }}
	}`
)

var ddb = []genieql.ColumnDefinition{
	{
		DBTypeName: "VARCHAR",
		Type:       "VARCHAR",
		ColumnType: "sql.NullString",
		Native:     stringExprString,
		Decode:     StdlibDecodeString,
		Encode:     StdlibEncodeString,
	},
	// {
	// 	DBTypeName: "VARCHAR[]",
	// 	Type:       "VARCHAR[]",
	// 	ColumnType: "sql.Null[[]string]",
	// 	Native:     stringArrExpr,
	// 	Decode:     StdlibDecodeNull,
	// 	Encode:     StdlibEncodeNull,
	// },
	{
		DBTypeName: "BOOLEAN",
		Type:       "BOOLEAN",
		ColumnType: "sql.NullBool",
		Native:     boolExprString,
		Decode:     StdlibDecodeBool,
		Encode:     StdlibEncodeBool,
	},
	{
		DBTypeName: "BIGINT",
		Type:       "BIGINT",
		ColumnType: "sql.NullInt64",
		Native:     int64ExprString,
		Decode:     StdlibDecodeInt64,
		Encode:     StdlibEncodeInt64,
	},
	{
		DBTypeName: "INTEGER",
		Type:       "INTEGER",
		ColumnType: "sql.NullInt32",
		Native:     int32ExprString,
		Decode:     StdlibDecodeInt32,
		Encode:     StdlibEncodeInt32,
	},
	{
		DBTypeName: "UINTEGER",
		Type:       "UINTEGER",
		ColumnType: "sql.NullInt64",
		Native:     uint32ExprString,
		Decode:     StdlibDecodeInt64,
		Encode:     StdlibEncodeInt64,
	},
	{
		DBTypeName: "UBIGINT",
		Type:       "UBIGINT",
		ColumnType: "ducktype.NullUint64",
		Native:     uint64ExprString,
		Decode:     StdlibDecodeUint64,
		Encode:     StdlibEncodeUint64,
	},
	{
		DBTypeName: "SMALLINT",
		Type:       "SMALLINT",
		ColumnType: "sql.NullInt16",
		Native:     int16ExprString,
		Decode:     StdlibDecodeInt16,
		Encode:     StdlibEncodeInt16,
	},
	// {
	// 	DBTypeName: "SMALLINT[]",
	// 	Type:       "SMALLINTARRAY",
	// 	ColumnType: "sql.Null[[]int]",
	// 	Native:     intArrExpr,
	// 	Decode:     StdlibDecodeNull,
	// 	Encode:     StdlibEncodeNull,
	// },
	{
		DBTypeName: "USMALLINT",
		Type:       "USMALLINT",
		ColumnType: "sql.NullInt32",
		Native:     uint16ExprString,
		Decode:     StdlibDecodeInt32,
		Encode:     StdlibEncodeInt32,
	},
	{
		DBTypeName: "FLOAT",
		Type:       "FLOAT",
		ColumnType: "sql.NullFloat64",
		Native:     float32ExprString,
		Decode:     StdlibDecodeFloat64,
		Encode:     StdlibEncodeFloat64,
	},
	{
		DBTypeName: "DOUBLE",
		Type:       "DOUBLE",
		ColumnType: "sql.NullFloat64",
		Native:     float64ExprString,
		Decode:     StdlibDecodeFloat64,
		Encode:     StdlibEncodeFloat64,
	},
	{
		DBTypeName: "UUID",
		Type:       "UUID",
		ColumnType: "sql.NullString",
		Native:     stringExprString,
		Decode:     ddbDecodeUUID,
		Encode:     StdlibEncodeString,
	},
	{
		DBTypeName: "TIMESTAMPZ",
		Type:       "TIMESTAMPZ",
		ColumnType: "ducktype.NullTime",
		Native:     timeExprString,
		Decode:     ddbDecodeTime,
		Encode:     ddbEncodeTime,
	},
	{
		DBTypeName: "BINARY",
		Type:       "BINARY",
		ColumnType: "[]byte",
		Native:     bytesExpr,
		Decode:     ddbDecodeBinary,
		Encode:     ddbEncodeBinary,
	},
	{
		DBTypeName: "BLOB",
		Type:       "BLOB",
		ColumnType: "[]byte",
		Native:     bytesExpr,
		Decode:     ddbDecodeBinary,
		Encode:     ddbEncodeBinary,
	},
	{
		DBTypeName: "INTERVAL",
		Type:       "INTERVAL",
		ColumnType: "ducktype.NullDuration",
		Native:     durationExpr,
		Decode:     StdlibDecodeDuration,
		Encode:     StdlibEncodeDuration,
	},
	{
		DBTypeName: "INET",
		Type:       "INET",
		ColumnType: "ducktype.NullNetAddr",
		Native:     netipAddrExpr,
		Decode:     StdlibDecodeNull,
		Encode:     StdlibEncodeNull,
	},
}
