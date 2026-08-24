package generators

import (
	"go/ast"
	"html/template"
	"io"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/internal/transformx"
)

// StructOption option to provide the structure function.
type StructOption func(*structure)

// StructOptionName provide the name of the struct to the structure.
func StructOptionName(n string) StructOption {
	return func(s *structure) {
		s.Name = n
	}
}

// StructOptionComment specify the comment for the structure
func StructOptionComment(comment *ast.CommentGroup) StructOption {
	return func(s *structure) {
		s.Comment = comment
	}
}

// StructOptionAliasStrategy provides the default aliasing strategy for
// generating the a struct's field names.
func StructOptionAliasStrategy(mcp genieql.MappingConfigOption) StructOption {
	return func(s *structure) {
		s.aliaser = mcp
	}
}

// StructOptionColumnsStrategy strategy for resolving column info for the structure.
func StructOptionColumnsStrategy(strategy columnsStrategy) StructOption {
	return func(s *structure) {
		s.columns = strategy
	}
}

// StructOptionTableStrategy convience function for creating a table based structure.
func StructOptionTableStrategy(table string) StructOption {
	return StructOptionColumnsStrategy(func(ctx Context) ([]genieql.ColumnInfo, error) {
		return ctx.Dialect.ColumnInformationForTable(ctx.Driver, table)
	})
}

// StructOptionQueryStrategy convience function for creating a query based structure.
func StructOptionQueryStrategy(query string) StructOption {
	return StructOptionColumnsStrategy(func(ctx Context) ([]genieql.ColumnInfo, error) {
		return ctx.Dialect.ColumnInformationForQuery(ctx.Driver, query)
	})
}

// StructOptionRenameMap provides explicit rename mappings when
// generating the struct's field names.
func StructOptionRenameMap(m map[string]string) StructOption {
	return func(s *structure) {
		s.renameMap = genieql.MCORenameMap(m)
	}
}

// StructOptionContext sets the Context for the structure generator.
func StructOptionContext(c Context) StructOption {
	return func(s *structure) {
		s.Context = c
	}
}

// StructOptionMappingConfigOptions sets the base configuration to be used for
// the MappingConfig.
func StructOptionMappingConfigOptions(options ...genieql.MappingConfigOption) StructOption {
	return func(s *structure) {
		s.mappingOptions = options
	}
}

// NewStructure creates a Generator that builds structures from column information.
func NewStructure(opts ...StructOption) genieql.Generator {
	s := structure{
		renameMap:      genieql.MCORenameMap(map[string]string{}),
		aliaser:        genieql.MCOTransformations("camelcase"),
		mappingOptions: []genieql.MappingConfigOption{},
	}

	for _, opt := range opts {
		opt(&s)
	}

	return s
}

type columnsStrategy func(Context) ([]genieql.ColumnInfo, error)
type structure struct {
	Context
	Name           string
	Comment        *ast.CommentGroup
	columns        columnsStrategy
	aliaser        genieql.MappingConfigOption
	renameMap      genieql.MappingConfigOption
	mappingOptions []genieql.MappingConfigOption
}

func (t structure) Generate(dst io.Writer) error {
	const tmpl = `type {{.Name}} struct {
	{{- range $column := .Columns }}
	{{ $column.Name | transformation }} {{ if $column.Definition.Nullable }}*{{ end }}{{ $column.Definition.Native -}}
	{{ end }}
}`
	type context struct {
		Name    string
		Columns []genieql.ColumnInfo
	}
	var (
		err     error
		columns []genieql.ColumnInfo
	)

	if columns, err = t.columns(t.Context); err != nil {
		return err
	}

	mapping := genieql.NewMappingConfig(
		append(
			t.mappingOptions,
			t.renameMap,
			t.aliaser,
			genieql.MCOColumns(columns...),
			genieql.MCOType(t.Name),
			genieql.MCOPackage(t.Context.CurrentPackage),
		)...,
	)

	if err = t.Context.Configuration.WriteMap(mapping); err != nil {
		return err
	}

	if err = GenerateComment(DefaultFunctionComment(t.Name), t.Comment).Generate(dst); err != nil {
		return err
	}

	ctx := context{
		Name:    t.Name,
		Columns: mapping.Columns,
	}

	a := mapping.Aliaser()

	return template.Must(template.New("scanner template").Funcs(template.FuncMap{
		"transformation": func(s string) string { return transformx.String(s, a) },
	}).Parse(tmpl)).Execute(dst, ctx)
}
