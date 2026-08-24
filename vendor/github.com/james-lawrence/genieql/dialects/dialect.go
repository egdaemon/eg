package dialects

import (
	"fmt"
	"strings"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/columninfo"
	"github.com/james-lawrence/genieql/internal/errorsx"
	"github.com/james-lawrence/genieql/internal/stringsx"
	"github.com/james-lawrence/genieql/internal/transformx"
	"golang.org/x/text/transform"
)

// ErrMissingDialect - returned when a dialect has not been registered.
type ErrMissingDialect interface {
	MissingDialect() string
}

// IsMissingDialectErr determines if the given error is a missing dialect error.
func IsMissingDialectErr(err error) bool {
	_, ok := err.(ErrMissingDialect)
	return ok
}

type errMissingDialect struct {
	dialect string
}

func (t errMissingDialect) MissingDialect() string {
	return t.dialect
}

func (t errMissingDialect) Error() string {
	return fmt.Sprintf("dialect (%s) is not registered", t.dialect)
}

// ErrDuplicateDialect - returned when a dialect gets registered twice.
var ErrDuplicateDialect = fmt.Errorf("dialect has already been registered")

var dialects = dialectRegistry{}

// Register register a sql dialect with genieql. usually in an init function.
func Register(dialect string, imp DialectFactory) error {
	return dialects.RegisterDialect(dialect, imp)
}

// LookupDialect lookup a registered dialect.
func LookupDialectByName(config genieql.Configuration) (genieql.Dialect, error) {
	var (
		err     error
		factory DialectFactory
	)

	if factory, err = dialects.LookupDialect(config.Dialect); err != nil {
		return nil, err
	}

	return factory.Connect(config)
}

// MustLookupDialect lookup a gesitered dialect or panic
func MustLookupDialect(c genieql.Configuration) genieql.Dialect {
	d, err := LookupDialect(c)
	errorsx.MaybePanic(err)

	return d
}

// DialectFactory ...
type DialectFactory interface {
	Connect(genieql.Configuration) (genieql.Dialect, error)
}

type dialectRegistry map[string]DialectFactory

func (t dialectRegistry) RegisterDialect(dialect string, imp DialectFactory) error {
	if _, exists := t[dialect]; exists {
		return ErrDuplicateDialect
	}

	t[dialect] = imp

	return nil
}

func (t dialectRegistry) LookupDialect(dialect string) (DialectFactory, error) {
	impl, exists := t[dialect]
	if !exists {
		return nil, errMissingDialect{dialect: dialect}
	}

	return impl, nil
}

type Test struct {
	Quote             string
	CValueTransformer genieql.ColumnTransformer
	QueryInsert       string
	QuerySelect       string
	QueryUpdate       string
	QueryDelete       string
}

func (t Test) Insert(n int, offset int, table, conflict string, columns, projection, defaults []string) string {
	var (
		insertTmpl = stringsx.DefaultIfBlank(t.QueryInsert, "INSERT QUERY")
	)
	offset = offset + 1
	values := make([]string, 0, n)
	for range n {
		var (
			p []string
		)
		p, newOffset := placeholders(offset, selectPlaceholder(columns, defaults))
		offset += newOffset
		values = append(values, fmt.Sprintf("(%s)", strings.Join(p, ",")))
	}

	columnOrder := strings.Join(columns, ",")

	replacements := strings.NewReplacer(
		":gql.insert.tablename:", table,
		":gql.insert.columns:", columnOrder,
		":gql.insert.values:", strings.Join(values, ","),
		":gql.insert.conflict:", stringsx.DefaultIfBlank(" "+conflict, ""),
		":gql.insert.returning:", columnOrder,
	)

	return replacements.Replace(insertTmpl)
}

func (t Test) Select(table string, columns, predicates []string) string {
	return stringsx.DefaultIfBlank(t.QuerySelect, "SELECT QUERY")
}

func (t Test) Update(table string, columns, predicates, returning []string) string {
	return stringsx.DefaultIfBlank(t.QueryUpdate, "UPDATE QUERY")
}

func (t Test) Delete(table string, columns, predicates []string) string {
	return stringsx.DefaultIfBlank(t.QueryDelete, "DELETE QUERY")
}

func (t Test) ColumnValueTransformer() genieql.ColumnTransformer {
	if t.CValueTransformer != nil {
		return t.CValueTransformer
	}

	return columninfo.StaticTransformer("?")
}

func (t Test) ColumnNameTransformer(opts ...transform.Transformer) genieql.ColumnTransformer {
	return columninfo.NewNameTransformer(transformx.Wrap(t.Quote), transform.Chain(opts...))
}

func (t Test) ColumnInformationForTable(d genieql.Driver, table string) ([]genieql.ColumnInfo, error) {
	mustLookupType := func(d genieql.ColumnDefinition, err error) genieql.ColumnDefinition {
		errorsx.MaybePanic(err)
		return d
	}

	switch table {
	case "struct_a":
		return []genieql.ColumnInfo{
			{Name: "a", Definition: mustLookupType(d.LookupType("int"))},
			{Name: "b", Definition: mustLookupType(d.LookupType("int"))},
			{Name: "c", Definition: mustLookupType(d.LookupType("int"))},
			{Name: "d", Definition: mustLookupType(d.LookupType("bool"))},
			{Name: "e", Definition: mustLookupType(d.LookupType("bool"))},
			{Name: "f", Definition: mustLookupType(d.LookupType("bool"))},
		}, nil
	default:
		return []genieql.ColumnInfo(nil), nil
	}
}

func (t Test) ColumnInformationForQuery(d genieql.Driver, query string) ([]genieql.ColumnInfo, error) {
	return []genieql.ColumnInfo(nil), nil
}

func (t Test) QuotedString(s string) string {
	return t.Quote + s + t.Quote
}

func placeholders(offset int, columns []placeholder) ([]string, int) {
	clauses := make([]string, 0, len(columns))
	idx := offset
	for _, column := range columns {
		var ph string
		ph, idx = column.String(idx)
		clauses = append(clauses, ph)
	}

	return clauses, len(clauses)
}

func selectPlaceholder(columns, defaults []string) []placeholder {
	placeholders := make([]placeholder, 0, len(columns))
	for _, column := range columns {
		var placeholder placeholder = offsetPlaceholder{}
		// todo turn into a set.
		for _, cut := range defaults {
			if cut == column {
				placeholder = defaultPlaceholder{}
				break
			}
		}
		placeholders = append(placeholders, placeholder)
	}

	return placeholders
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

type TestFactory Test

func (t TestFactory) Connect(genieql.Configuration) (genieql.Dialect, error) {
	return genieql.Dialect(Test(t)), nil
}
