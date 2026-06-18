package genieql

import (
	"golang.org/x/text/transform"
)

// Dialect interface for describing a sql dialect.
type Dialect interface {
	Insert(n int, offset int, table string, conflict string, columns, projection, defaults []string) string
	Select(table string, columns, predicates []string) string
	Update(table string, columns, predicates, returning []string) string
	Delete(table string, columns, predicates []string) string
	ColumnValueTransformer() ColumnTransformer
	ColumnNameTransformer(opts ...transform.Transformer) ColumnTransformer
	ColumnInformationForTable(d Driver, table string) ([]ColumnInfo, error)
	ColumnInformationForQuery(d Driver, query string) ([]ColumnInfo, error)
	QuotedString(s string) string
}
