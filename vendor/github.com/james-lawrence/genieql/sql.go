package genieql

import (
	"fmt"
	"go/ast"
	"sort"

	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/internal/stringsx"
	"golang.org/x/text/transform"
)

// ColumnInfo describing a particular column from the database.
// include the column definition, the column name.
type ColumnInfo struct {
	Definition ColumnDefinition
	Name       string
}

// MapColumn map the column to a particular expression.
func (t ColumnInfo) MapColumn(x ast.Expr) ColumnMap {
	return ColumnMap{
		ColumnInfo: t,
		Dst:        x,
	}
}

type lesser func(i, j ColumnInfo) bool

// SortColumnInfo ...
func SortColumnInfo(input []ColumnInfo) func(c lesser) []ColumnInfo {
	return func(c lesser) []ColumnInfo {
		sort.Slice(input, func(i, j int) bool {
			return c(input[i], input[j])
		})
		return input
	}
}

// ByName comparison for ColumnInfo
func ByName(i, j ColumnInfo) bool {
	return i.Name < j.Name
}

// ColumnInfoSet a set of columns
type ColumnInfoSet []ColumnInfo

// ColumnNames returns the column names inside the ColumnInfoSet.
func (t ColumnInfoSet) ColumnNames() []string {
	var columns []string

	for _, column := range t {
		columns = append(columns, column.Name)
	}

	return columns
}

// Filter filters out columns in the set based on the filter function.
func (t ColumnInfoSet) Filter(cut func(ColumnInfo) bool) ColumnInfoSet {
	result := make([]ColumnInfo, 0, len(t))
	for _, column := range t {
		if cut(column) {
			result = append(result, column)
		}
	}

	return ColumnInfoSet(result)
}

// PrimaryKey - returns the primary key from the column set.
func (t ColumnInfoSet) PrimaryKey() ColumnInfoSet {
	return t.Filter(PrimaryKeyFilter)
}

// PrimaryKeyFilter - selects ColumnInfo which are part of the primary key.
func PrimaryKeyFilter(column ColumnInfo) bool {
	return column.Definition.PrimaryKey
}

// NotPrimaryKeyFilter - inverse of PrimaryKeyFilter
func NotPrimaryKeyFilter(column ColumnInfo) bool {
	return columnInfoNotFilter(PrimaryKeyFilter)(column)
}

func columnInfoNotFilter(x func(ColumnInfo) bool) func(ColumnInfo) bool {
	return func(c ColumnInfo) bool {
		return !x(c)
	}
}

// AmbiguityCheck checks the provided columns for duplicated values.
func (t ColumnInfoSet) AmbiguityCheck() error {
	var (
		columnNames = t.ColumnNames()
	)

	sort.Strings(columnNames)

	ambiguousColumns := []string{}

	if len(columnNames) > 0 {
		previous, tail := columnNames[0], columnNames[1:]
		lastMatch := ""
		for _, current := range tail {
			if previous == current && lastMatch != current {
				ambiguousColumns = append(ambiguousColumns, current)
				lastMatch = current
			}
			previous = current
		}
	}

	if len(ambiguousColumns) > 0 {
		return fmt.Errorf("ambiguous columns in results %v", ambiguousColumns)
	}

	return nil
}

// ColumnInfoFilterIgnore filter that ignores column with a name in the set.
func ColumnInfoFilterIgnore(set ...string) func(ColumnInfo) bool {
	return func(c ColumnInfo) bool {
		for _, ignore := range set {
			if ignore == c.Name {
				return false
			}
		}

		return true
	}
}

// ColumnValueTransformer ...
type ColumnValueTransformer struct {
	Defaults           []string
	DialectTransformer ColumnTransformer
}

// Transform ...
func (t ColumnValueTransformer) Transform(column ColumnInfo) string {
	const defaultValue = "DEFAULT"
	if stringsx.Contains(column.Name, t.Defaults...) {
		return defaultValue
	}
	return t.DialectTransformer.Transform(column)
}

// ColumnTransformer transforms a ColumnInfo into a string for the constant.
type ColumnTransformer interface {
	Transform(ColumnInfo) string
}

// TableDetails provides information about the table.
type TableDetails struct {
	Dialect         Dialect
	Table           string
	Columns         []ColumnInfo
	UnmappedColumns []ColumnInfo
}

// LookupTableDetails determines the table details for the given dialect.
func LookupTableDetails(driver Driver, dialect Dialect, table string) (TableDetails, error) {
	var (
		err     error
		columns []ColumnInfo
	)

	if columns, err = dialect.ColumnInformationForTable(driver, table); err != nil {
		return TableDetails{}, err
	}

	return TableDetails{
		Dialect: dialect,
		Table:   table,
		Columns: columns,
	}, nil
}

// mapColumns maps the columns to the fields using the provided function.
// returns mapped, unmapped columns.
func mapColumns(columns []ColumnInfo, fields []*ast.Field, m func(ColumnInfo, *ast.Field) *ColumnMap) ([]ColumnMap, []ColumnInfo) {
	if len(fields) == 0 {
		return []ColumnMap{}, columns
	}

	mColumns := make([]ColumnMap, 0, len(columns))
	uColumns := make([]ColumnInfo, 0, len(columns))

	for _, column := range columns {
		var matched *ColumnMap
		for _, field := range fields {
			if matched = m(column, field); matched != nil {
				break
			}
		}

		if matched != nil {
			mColumns = append(mColumns, *matched)
		} else {
			uColumns = append(uColumns, column)
		}
	}

	return mColumns, uColumns
}

// mapInfo maps the columns to the fields using the aliases.
// returns mapped, unmapped columns.
func mapInfo(columns []ColumnInfo, fields []*ast.Field, aliases ...transform.Transformer) ([]ColumnInfo, []ColumnInfo) {
	if len(fields) == 0 {
		return []ColumnInfo{}, columns
	}

	mColumns := make([]ColumnInfo, 0, len(columns))
	uColumns := make([]ColumnInfo, 0, len(columns))

	for _, column := range columns {
		var matched *ast.Field
		for _, field := range fields {
			if matched = MapFieldToNativeType(column, field, aliases...); matched != nil {
				break
			}
		}

		if matched != nil {
			mColumns = append(mColumns, column)
		} else {
			uColumns = append(uColumns, column)
		}
	}

	return mColumns, uColumns
}

func mapFields(columns []ColumnInfo, fields []*ast.Field, mp func(ColumnInfo, *ast.Field) *ast.Field) ([]*ast.Field, []*ast.Field) {
	if len(fields) == 0 {
		return []*ast.Field{}, fields
	}

	if len(columns) == 0 {
		return []*ast.Field{}, fields
	}

	mFields := make([]*ast.Field, 0, len(fields))
	uFields := make([]*ast.Field, 0, len(fields))

	for _, field := range astutil.FlattenFields(fields...) {
		var (
			matched *ast.Field
		)

		for _, column := range columns {
			if matched = mp(column, field); matched != nil {
				break
			}
		}

		if matched != nil {
			mFields = append(mFields, matched)
		} else {
			uFields = append(uFields, field)
		}
	}

	return mFields, uFields
}
