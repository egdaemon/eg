package drivers

import (
	"io"

	yaml "gopkg.in/yaml.v3"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/internal/errorsx"
)

// DefaultTypeDefinitions determine the type definition for an expression.
func DefaultTypeDefinitions(s string) (genieql.ColumnDefinition, error) {
	return stdlib.LookupType(s)
}

type config struct {
	Name  string
	Types []genieql.ColumnDefinition
}

// ReadDriver - reads a driver from an io.Reader
func ReadDriver(in io.Reader) (name string, driver genieql.Driver, err error) {
	var (
		raw    []byte
		config config
	)

	if raw, err = io.ReadAll(in); err != nil {
		return "",
			nil, errorsx.Wrap(err, "failed to read driver")
	}

	if err = yaml.Unmarshal(raw, &config); err != nil {
		return "",
			nil, errorsx.Wrap(err, "failed to unmarshal driver")
	}

	return config.Name, NewDriver("", config.Types...), nil
}

// NewDriver build a driver from the nullable types
func NewDriver(path string, types ...genieql.ColumnDefinition) genieql.Driver {
	return genieql.NewDriver(path, types...)
}

func init() {
	errorsx.MaybePanic(genieql.RegisterDriver(StandardLib, stdlib))
}

// StandardLib driver only uses types from stdlib.
const StandardLib = "genieql.default"

const (
	StdlibEncodeString = `func() {
		{{ .To | expr }}.Valid = true
		{{ .To | expr }}.String = {{ .From | expr }}
	}`
	StdlibDecodeString = `func() {
		if {{ .From | expr }}.Valid {
			tmp := {{ .Type | expr }}({{ .From | expr }}.String)
			{{ .To | autodereference | expr }} = tmp
		}
	}`
	StdlibEncodeInt32 = `func() {
		{{ .To | expr }}.Valid = true
		{{ .To | expr }}.Int32 = int32({{ .From | expr }})
	}`
	StdlibDecodeInt32 = `func() {
		if {{ .From | expr }}.Valid {
			tmp := {{ .Type | expr }}({{ .From | expr }}.Int32)
			{{ .To | autodereference | expr }} = tmp
		}
	}`
	StdlibEncodeBool = `func() {
		{{ .To | expr }}.Valid = true
		{{ .To | expr }}.Bool = {{ .From | expr }}
	}`
	StdlibDecodeBool = `func() {
		if {{ .From | expr }}.Valid {
			tmp := {{ .From | expr }}.Bool
			{{ .To | autodereference | expr }} = tmp
		}
	}`
	StdlibEncodeFloat64 = `func() {
		{{ .To | expr }}.Valid = true
		{{ .To | expr }}.Float64 = float64({{ .From | expr }})
	}`
	StdlibDecodeFloat64 = `func() {
		if {{ .From | expr }}.Valid {
			tmp := {{ .Type | expr }}({{ .From | expr }}.Float64)
			{{ .To | autodereference | expr }} = tmp
		}
	}`
	StdlibEncodeTime = `func() {
		{{ .To | expr }}.Valid = true
		{{ .To | expr }}.Time = {{ .From | expr }}
	}`
	StdlibDecodeTime = `func() {
		if {{ .From | expr }}.Valid {
			tmp := {{ .From | expr }}.Time
			{{ .To | autodereference | expr }} = tmp
		}
	}`
	StdlibEncodeDuration = `func() {
		{{ .To | expr }}.Valid = true
		{{ .To | expr }}.V = {{ .From | expr }}
	}`
	StdlibDecodeDuration = `func() {
		if {{ .From | expr }}.Valid {
			tmp := {{ .From | expr }}.V
			{{ .To | autodereference | expr }} = tmp
		}
	}`
	StdlibEncodeInt64 = `func() {
		{{ .To | expr }}.Valid = true
		{{ .To | expr }}.Int64 = int64({{ .From | expr }})
	}`
	StdlibDecodeInt64 = `func() {
		if {{ .From | expr }}.Valid {
			tmp := {{ .Type | expr }}({{ .From | expr }}.Int64)
			{{ .To | autodereference | expr }} = tmp
		}
	}`
	StdlibEncodeUint64 = `func() {
		{{ .To | expr }}.Valid = true
		{{ .To | expr }}.V = {{ .From | expr }}
	}`
	StdlibDecodeUint64 = `func() {
		if {{ .From | expr }}.Valid {
			tmp := {{ .From | expr }}.V
			{{ .To | autodereference | expr }} = tmp
		}
	}`
	StdlibEncodeInt16 = `func() {
		{{ .To | expr }}.Valid = true
		{{ .To | expr }}.Int16 = int16({{ .From | expr }})
	}`
	StdlibDecodeInt16 = `func() {
		if {{ .From | expr }}.Valid {
			tmp := {{ .Type | expr }}({{ .From | expr }}.Int16)
			{{ .To | autodereference | expr }} = tmp
		}
	}`
	StdlibEncodeNull = `func() {
		{{ .To | expr }}.Valid = true
		{{ .To | expr }}.V = {{ .From | expr }}
	}`
	StdlibDecodeNull = `func() {
		if {{ .From | expr }}.Valid {
			{{ .To | autodereference | expr }} = {{ .From | expr }}.V
		}
	}`
)

var stdlib = NewDriver(
	StandardLib,
	genieql.ColumnDefinition{
		Type:       "sql.NullString",
		ColumnType: "sql.NullString",
		Native:     stringExprString,
		Decode:     StdlibDecodeString,
		Encode:     StdlibEncodeString,
	},
	genieql.ColumnDefinition{
		Type:       "sql.NullInt32",
		ColumnType: "sql.NullInt32",
		Native:     int32ExprString,
		Decode:     StdlibDecodeInt32,
		Encode:     StdlibEncodeInt32,
	},
	genieql.ColumnDefinition{
		Type:       "sql.NullFloat64",
		ColumnType: "sql.NullFloat64",
		Native:     float64ExprString,
		Decode:     StdlibDecodeFloat64,
		Encode:     StdlibEncodeFloat64,
	},
	genieql.ColumnDefinition{
		Type:       "sql.NullBool",
		ColumnType: "sql.NullBool",
		Native:     boolExprString,
		Decode:     StdlibDecodeBool,
		Encode:     StdlibEncodeBool,
	},
	genieql.ColumnDefinition{
		Type:       "sql.NullTime",
		ColumnType: "sql.NullTime",
		Native:     timeExprString,
		Decode:     StdlibDecodeTime,
		Encode:     StdlibEncodeTime,
	},
	genieql.ColumnDefinition{
		Type:       "int",
		ColumnType: "sql.NullInt64",
		Native:     intExprString,
		Decode:     StdlibDecodeInt64,
		Encode:     StdlibEncodeInt64,
	},
	genieql.ColumnDefinition{
		Type:       "*int",
		Nullable:   true,
		Native:     intExprString,
		ColumnType: "sql.NullInt64",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .Type | expr }}({{ .From | expr }}.Int64)
				{{ .To | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Int64 = int64({{ .From | expr }})
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "int32",
		Native:     int32ExprString,
		ColumnType: "sql.NullInt32",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.Int32
				{{ .To | autodereference | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Int32 = int32({{ .From | expr }})
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "*int32",
		Nullable:   true,
		Native:     int32ExprString,
		ColumnType: "sql.NullInt32",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.Int32
				{{ .To | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Int32 = int32({{ .From | expr }})
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "sql.NullInt64",
		ColumnType: "sql.NullInt64",
		Native:     int64ExprString,
		Decode:     StdlibDecodeInt64,
		Encode:     StdlibEncodeInt64,
	},
	genieql.ColumnDefinition{
		Type:       "int64",
		ColumnType: "sql.NullInt64",
		Native:     int64ExprString,
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.Int64
				{{ .To | autodereference | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Int64 = {{ .From | expr }}
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "*int64",
		ColumnType: "sql.NullInt64",
		Native:     intExprString,
		Nullable:   true,
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.Int64
				{{ .To | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Int64 = {{ .From | expr }}
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "float32",
		Native:     float64ExprString,
		ColumnType: "sql.NullFloat64",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .Type | expr}}({{ .From | expr }}.Float64)
				{{ .To | autodereference | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Float64 = float64({{ .From | expr }})
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "*float32",
		Nullable:   true,
		Native:     float64ExprString,
		ColumnType: "sql.NullFloat64",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .Type | expr}}({{ .From | expr }}.Float64)
				{{ .To | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Float64 = float64({{ .From | expr }})
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "float64",
		Native:     float64ExprString,
		ColumnType: "sql.NullFloat64",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.Float64
				{{ .To | autodereference | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Float64 = {{ .From | expr }}
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "*float64",
		Nullable:   true,
		Native:     float64ExprString,
		ColumnType: "sql.NullFloat64",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.Float64
				{{ .To | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Float64 = {{ .From | expr }}
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "bool",
		Native:     boolExprString,
		ColumnType: "sql.NullBool",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.Bool
				{{ .To | autodereference | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Bool = {{ .From | expr }}
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "*bool",
		Nullable:   true,
		Native:     boolExprString,
		ColumnType: "sql.NullBool",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.Bool
				{{ .To | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Bool = {{ .From | expr }}
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "time.Time",
		Native:     timeExprString,
		ColumnType: "sql.NullTime",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.Time
				{{ .To | autodereference | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Time = {{ .From | expr }}
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "*time.Time",
		Nullable:   true,
		Native:     timeExprString,
		ColumnType: "sql.NullTime",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.Time
				{{ .To | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.Time = {{ .From | expr }}
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "string",
		Native:     stringExprString,
		ColumnType: "sql.NullString",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.String
				{{ .To | autodereference | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.String = {{ .From | expr }}
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "*string",
		Nullable:   true,
		Native:     stringExprString,
		ColumnType: "sql.NullString",
		Decode: `func() {
			if {{ .From | expr }}.Valid {
				tmp := {{ .From | expr }}.String
				{{ .To | expr }} = tmp
			}
		}`,
		Encode: `func() {
			{{ .To | expr }}.Valid = true
			{{ .To | expr }}.String = {{ .From | expr }}
		}`,
	},
	genieql.ColumnDefinition{
		Type:       "uint16",
		Native:     uint16ExprString,
		ColumnType: "sql.Null[uint16]",
		Decode:     StdlibDecodeNull,
		Encode:     StdlibEncodeNull,
	},
)
