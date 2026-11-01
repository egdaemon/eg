package postgresql

import (
	"fmt"
	"strings"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/internal/stringsx"
)

type ColumnValueTransformer struct {
	offset int
}

func (t *ColumnValueTransformer) Transform(c genieql.ColumnInfo) string {
	t.offset++
	p, _ := offsetPlaceholder{}.String(t.offset)
	return p
}

// Insert generate an insert query.
func Insert(n int, offset int, table string, conflict string, columns, projection, defaulted []string) string {
	const (
		insertTmpl = "INSERT INTO :gql.insert.tablename: (:gql.insert.columns:) VALUES :gql.insert.values::gql.insert.conflict: RETURNING :gql.insert.returning:"
	)

	columnOrder := strings.Join(quotedColumns(projection...), ",")
	insertions := strings.Join(quotedColumns(columns...), ",")
	offset = offset + 1
	values := make([]string, 0, n)
	for i := 0; i < n; i++ {
		var (
			p []string
		)
		p, offset = placeholders(offset, selectPlaceholder(columns, defaulted))
		values = append(values, fmt.Sprintf("(%s)", strings.Join(p, ",")))
	}

	replacements := strings.NewReplacer(
		":gql.insert.tablename:", table,
		":gql.insert.columns:", insertions,
		":gql.insert.values:", strings.Join(values, ","),
		":gql.insert.conflict:", stringsx.DefaultIfBlank(" "+conflict, ""),
		":gql.insert.returning:", columnOrder,
	)

	return replacements.Replace(insertTmpl)
}

// Select generate a select query.
func Select(table string, columns, predicates []string) string {
	clauses, _ := predicate(1, predicates...)
	columnOrder := strings.Join(quotedColumns(columns...), ",")
	return fmt.Sprintf(selectByFieldTmpl, columnOrder, table, strings.Join(clauses, " AND "))
}

// Update generate an update query.
func Update(table string, columns, predicates, returning []string) string {
	updates, offset := predicate(1, columns...)
	clauses, _ := predicate(offset, predicates...)
	columnOrder := strings.Join(quotedColumns(returning...), ",")
	return fmt.Sprintf(updateTmpl, table, strings.Join(updates, ", "), strings.Join(clauses, " AND "), columnOrder)
}

// Delete generate a delete query.
func Delete(table string, columns, predicates []string) string {
	clauses, _ := predicate(1, predicates...)
	columnOrder := strings.Join(quotedColumns(columns...), ",")
	return fmt.Sprintf(deleteTmpl, table, strings.Join(clauses, " AND "), columnOrder)
}

func predicate(offset int, predicates ...string) ([]string, int) {
	clauses := make([]string, 0, len(predicates))
	for idx, predicate := range quotedColumns(predicates...) {
		clauses = append(clauses, fmt.Sprintf("%s = $%d", predicate, offset+idx))
	}

	if len(clauses) == 0 {
		clauses = append(clauses, matchAllClause)
	}

	return clauses, len(predicates) + 1
}

func placeholders(offset int, columns []placeholder) ([]string, int) {
	clauses := make([]string, 0, len(columns))
	idx := offset
	for _, column := range columns {
		var ph string
		ph, idx = column.String(idx)
		clauses = append(clauses, ph)
	}

	return clauses, idx
}

func selectPlaceholder(columns, defaults []string) []placeholder {
	placeholders := make([]placeholder, 0, len(columns))
	defaulted := make(map[string]struct{}, len(defaults))
	for _, d := range defaults {
		defaulted[d] = struct{}{}
	}

	for _, column := range columns {
		var placeholder placeholder = offsetPlaceholder{}
		if _, ok := defaulted[column]; ok {
			placeholder = defaultPlaceholder{}
		}
		placeholders = append(placeholders, placeholder)
	}

	return placeholders
}

func quotedString(s string) string {
	return `"` + s + `"`
}

func quotedColumns(columns ...string) []string {
	results := make([]string, 0, len(columns))
	for _, c := range columns {
		results = append(results, quotedString(c))
	}
	return results
}

type placeholder interface {
	String(offset int) (string, int)
}

type defaultPlaceholder struct{}

func (t defaultPlaceholder) String(offset int) (string, int) {
	return "DEFAULT", offset
}

type offsetPlaceholder struct{}

func (t offsetPlaceholder) String(offset int) (string, int) {
	return fmt.Sprintf("$%d", offset), offset + 1
}

const selectByFieldTmpl = "SELECT %s FROM %s WHERE %s"
const updateTmpl = "UPDATE %s SET %s WHERE %s RETURNING %s"
const deleteTmpl = "DELETE FROM %s WHERE %s RETURNING %s"
const matchAllClause = "'t'"
