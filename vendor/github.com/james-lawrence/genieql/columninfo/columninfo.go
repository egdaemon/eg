package columninfo

import (
	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/internal/transformx"
	"golang.org/x/text/transform"
)

// Rename - applies transformations
func Rename(column genieql.ColumnInfo, m transform.Transformer) (string, error) {
	r, _, err := transform.String(m, column.Name)
	return r, err
}

// NewNameTransformer ColumnNameTransform implementation.
func NewNameTransformer(transforms ...transform.Transformer) NameTransformer {
	return NameTransformer{m: transform.Chain(transforms...)}
}

// NameTransformer ...
type NameTransformer struct {
	m transform.Transformer
}

// Transform applies a transformation to the column name.
func (t NameTransformer) Transform(column genieql.ColumnInfo) string {
	return transformx.String(column.Name, t.m)
}

// StaticTransformer always returns the same value.
type StaticTransformer string

// Transform applies a transformation to the column name.
func (t StaticTransformer) Transform(column genieql.ColumnInfo) string {
	return string(t)
}
