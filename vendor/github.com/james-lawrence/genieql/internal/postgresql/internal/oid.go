package internal

import (
	"go/ast"

	"github.com/jackc/pgtype"

	"github.com/james-lawrence/genieql/astutil"
)

// OIDToType maps object id to golang types.
func OIDToType(oid int) ast.Expr {
	switch oid {
	case pgtype.BoolOID:
		return astutil.Expr("pgtype.Bool")
	case pgtype.UUIDOID:
		return astutil.Expr("pgtype.UUID")
	case pgtype.BPCharOID:
		return astutil.Expr("pgtype.BPChar")
	case pgtype.UUIDArrayOID:
		return astutil.Expr("pgtype.UUIDArray")
	case pgtype.IntervalOID:
		return astutil.Expr("pgtype.Interval")
	case pgtype.CIDROID:
		return astutil.Expr("pgtype.CIDR")
	case pgtype.CIDRArrayOID:
		return astutil.Expr("pgtype.CIDRArray")
	case pgtype.MacaddrOID:
		return astutil.Expr("pgtype.Macaddr")
	case pgtype.TimestamptzOID:
		return astutil.Expr("pgtype.Timestamptz")
	case pgtype.TimestampOID:
		return astutil.Expr("pgtype.Timestamp")
	case pgtype.DateOID:
		return astutil.Expr("pgtype.Date")
	case pgtype.BitOID:
		return astutil.Expr("pgtype.Bit")
	case pgtype.VarbitOID:
		return astutil.Expr("pgtype.Varbit")
	case pgtype.Int2ArrayOID:
		return astutil.Expr("pgtype.Int2Array")
	case pgtype.Int4ArrayOID:
		return astutil.Expr("pgtype.Int4Array")
	case pgtype.Int8ArrayOID:
		return astutil.Expr("pgtype.Int8Array")
	case pgtype.Int2OID:
		return astutil.Expr("pgtype.Int2")
	case pgtype.Int4OID:
		return astutil.Expr("pgtype.Int4")
	case pgtype.Int8OID:
		return astutil.Expr("pgtype.Int8")
	case pgtype.TextOID:
		return astutil.Expr("pgtype.Text")
	case pgtype.TextArrayOID:
		return astutil.Expr("pgtype.TextArray")
	case pgtype.VarcharOID:
		return astutil.Expr("pgtype.Varchar")
	case pgtype.JSONOID:
		return astutil.Expr("pgtype.JSON")
	case pgtype.JSONBOID:
		return astutil.Expr("pgtype.JSONB")
	case pgtype.ByteaOID:
		return astutil.Expr("pgtype.Bytea")
	case pgtype.Float4OID:
		return astutil.Expr("pgtype.Float4")
	case pgtype.Float8OID:
		return astutil.Expr("pgtype.Float8")
	case pgtype.NumericOID:
		return astutil.Expr("pgtype.Numeric")
	case pgtype.InetOID:
		return astutil.Expr("pgtype.Inet")
	case pgtype.OIDOID:
		return astutil.Expr("pgtype.OID")
	case pgtype.NameOID:
		return astutil.Expr("pgtype.Name")
	default:
		return nil
	}
}
