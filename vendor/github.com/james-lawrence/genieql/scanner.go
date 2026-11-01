package genieql

import (
	"fmt"
	"go/ast"
)

type ColumnMap struct {
	ColumnInfo
	Dst   ast.Expr
	Field *ast.Field
}

func (t ColumnMap) Local(i int) *ast.Ident {
	return &ast.Ident{
		Name: fmt.Sprintf("c%d", i),
	}
}

// ColumnMapSet a set of column mappings
type ColumnMapSet []ColumnMap

// Filter filters out columns in the set based on the filter function.
func (t ColumnMapSet) Filter(cut func(ColumnMap) bool) ColumnMapSet {
	result := make([]ColumnMap, 0, len(t))
	for _, column := range t {
		if cut(column) {
			result = append(result, column)
		}
	}

	return ColumnMapSet(result)
}

// ColumnNames returns the column names inside the ColumnMapSet.
func (t ColumnMapSet) ColumnNames() (columns []string) {
	columns = make([]string, 0, len(t))
	for _, column := range t {
		columns = append(columns, column.ColumnInfo.Name)
	}
	return columns
}

func (t ColumnMapSet) ColumnInfo() (result ColumnInfoSet) {
	result = make([]ColumnInfo, 0, len(t))
	for _, cm := range t {
		result = append(result, cm.ColumnInfo)
	}

	return result
}

func (t ColumnMapSet) Map(m func(int, ColumnMap) ColumnMap) (result ColumnMapSet) {
	result = make(ColumnMapSet, 0, len(t))
	for idx, cm := range t {
		result = append(result, m(idx, cm))
	}
	return result
}
