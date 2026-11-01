package ginterp

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/james-lawrence/genieql"
	// register the drivers

	"github.com/james-lawrence/genieql/generators"
	_ "github.com/james-lawrence/genieql/internal/drivers"
	// register the postgresql dialect
	_ "github.com/james-lawrence/genieql/internal/postgresql"
	// register the wasi dialect
	_ "github.com/james-lawrence/genieql/internal/wasidialect"
)

type definition interface {
	Columns() ([]genieql.ColumnInfo, error)
}

// quotedTableName quotes each part of a possibly schema-qualified table
// reference individually via the dialect's own QuotedString, then joins them
// with ".". A single part is unaffected structurally (still just one quoted
// identifier); multiple parts (e.g. "cache", "library_known_media") produce
// a valid two-part qualified reference ("cache"."library_known_media").
func quotedTableName(d genieql.Dialect, parts ...string) string {
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		quoted = append(quoted, d.QuotedString(p))
	}
	return strings.Join(quoted, ".")
}

// Query extracts table information from the database making it available for
// further processing.
func Query(driver genieql.Driver, dialect genieql.Dialect, query string) QueryInfo {
	return QueryInfo{
		Driver:  driver,
		Dialect: dialect,
		Query:   query,
	}
}

// QueryInfo ...
type QueryInfo struct {
	Driver  genieql.Driver
	Dialect genieql.Dialect
	Query   string
}

// Columns ...
func (t QueryInfo) Columns() ([]genieql.ColumnInfo, error) {
	return t.Dialect.ColumnInformationForQuery(t.Driver, t.Query)
}

// Table extracts table information from the database making it available for
// further processing.
func Table(driver genieql.Driver, d genieql.Dialect, name string) TableInfo {
	return TableInfo{
		Driver:  driver,
		Dialect: d,
		Name:    name,
	}
}

// TableInfo ...
type TableInfo struct {
	Driver  genieql.Driver
	Dialect genieql.Dialect
	Name    string
}

// Columns ...
func (t TableInfo) Columns() ([]genieql.ColumnInfo, error) {
	return t.Dialect.ColumnInformationForTable(t.Driver, t.Name)
}

// Camelcase the column name.
func Camelcase(c genieql.ColumnInfo) genieql.ColumnInfo {
	return c
}

// Snakecase the column name.
func Snakecase(c genieql.ColumnInfo) genieql.ColumnInfo {
	return c
}

// Lowercase the column name.
func Lowercase(c genieql.ColumnInfo) genieql.ColumnInfo {
	return c
}

// Uppercase the column name.
func Uppercase(c genieql.ColumnInfo) genieql.ColumnInfo {
	return c
}

func nodeInfo(ctx generators.Context, n ast.Node) string {
	pos := ctx.FileSet.PositionFor(n.Pos(), true).String()
	switch n := n.(type) {
	case *ast.FuncDecl:
		return fmt.Sprintf("(%s.%s - %s)", ctx.CurrentPackage.Name, n.Name, pos)
	default:
		return fmt.Sprintf("(%s.%T - %s)", ctx.CurrentPackage.Name, n, pos)
	}
}
