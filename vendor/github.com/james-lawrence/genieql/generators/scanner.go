package generators

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/types"
	"io"
	"log"
	"strings"
	"text/template"

	"golang.org/x/text/transform"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/internal/errorsx"
	"github.com/james-lawrence/genieql/internal/stringsx"
)

type mode int

func (t mode) Enabled(o mode) bool {
	return t&o != 0
}

func (t mode) Disabled(o mode) bool {
	return t&o == 0
}

const (
	// ModeInterface - output the scanner interface.
	ModeInterface mode = 1 << iota
	// ModeStatic - output the static scanner.
	ModeStatic
	// ModeStaticDisableColumns - do not output the columns for the static scanner.
	ModeStaticDisableColumns
	// ModeDynamic - output the dynamic scanner.
	ModeDynamic
)

// ScannerOption option to provide the structure function.
type ScannerOption func(*scanner) error

// ScannerOptionNoop changes nothing
func ScannerOptionNoop(s *scanner) error {
	return nil
}

// ScannerOptionName provide the base name for the scanners.
func ScannerOptionName(n string) ScannerOption {
	return func(s *scanner) error {
		s.Name = n
		return nil
	}
}

// ScannerOptionParameters provide the parameters for the scanner.
func ScannerOptionParameters(fields *ast.FieldList) ScannerOption {
	// ensure each parameter has at least 1 name
	for idx, field := range fields.List {
		if len(field.Names) == 0 {
			field.Names = []*ast.Ident{ast.NewIdent(fmt.Sprintf("sp%d", idx))}
			fields.List[idx] = field
		}
	}

	return func(s *scanner) error {
		s.Fields = fields
		return nil
	}
}

// ScannerOptionsFromComment extracts configuration from a comment group attached
// to the function type.
func ScannerOptionsFromComment(comment *ast.CommentGroup) []ScannerOption {
	// TODO
	return []ScannerOption{}
}

// ScannerOptionPackage provides the package being used to build the scanners.
func ScannerOptionPackage(p *build.Package) ScannerOption {
	return func(s *scanner) error {
		s.Context.CurrentPackage = p
		return nil
	}
}

// ScannerOptionContext set the generation context for the scanner.
func ScannerOptionContext(p Context) ScannerOption {
	return func(s *scanner) error {
		s.Context = p
		return nil
	}
}

// ScannerOptionConfiguration provides the configuration being used to build the scanners.
func ScannerOptionConfiguration(c genieql.Configuration) ScannerOption {
	return func(s *scanner) error {
		s.Context.Configuration = c
		return nil
	}
}

// ScannerOptionOutputMode set the output mode for the scanner.
// e.g.) generators.ModeInterface|generators.ModeStatic|generators.ModeDynamic
// each mode provided enables the given scanner type.
func ScannerOptionOutputMode(m mode) ScannerOption {
	return func(s *scanner) error {
		s.Mode = m
		return nil
	}
}

// ScannerOptionEnableMode enable the output mode.
func ScannerOptionEnableMode(m mode) ScannerOption {
	return func(s *scanner) error {
		s.Mode = s.Mode | m
		return nil
	}
}

// ScannerOptionInterfaceName DEPRECATED only used for old functions.
func ScannerOptionInterfaceName(n string) ScannerOption {
	return func(s *scanner) error {
		s.interfaceName = n
		return nil
	}
}

// ScannerOptionIgnoreSet DEPRECATED only used for old functions.
// sets the selection of columns to be ignored.
func ScannerOptionIgnoreSet(n ...string) ScannerOption {
	return func(s *scanner) error {
		s.ignoreSet = n
		return nil
	}
}

// ScannerOptionColumnNameTransformer specify transforms for the column name
func ScannerOptionColumnNameTransformer(n ...transform.Transformer) ScannerOption {
	return func(s *scanner) error {
		s.columnNameTransformer = n
		return nil
	}
}

// ScannerFromGenDecl creates a structure generator from  from the provided *ast.GenDecl
func ScannerFromGenDecl(decl *ast.GenDecl, providedOptions ...ScannerOption) []genieql.Generator {
	g := make([]genieql.Generator, 0, len(decl.Specs)*2)

	for _, spec := range decl.Specs {
		if ts, ok := spec.(*ast.TypeSpec); ok {
			if ft, ok := ts.Type.(*ast.FuncType); ok {
				options := []ScannerOption{
					ScannerOptionName(ts.Name.Name),
					ScannerOptionParameters(ft.Params),
				}

				if len(ft.Params.List) > 1 && !allBuiltinTypes(astutil.MapFieldsToTypeExpr(ft.Params.List...)...) {
					log.Println("multiple structures detected disabling dynamic scanner output for", ts.Name.Name, types.ExprString(ft), ":", genieql.PrintDebug())
					options = append(options, ScannerOptionOutputMode(ModeInterface|ModeStatic|ModeStaticDisableColumns))
				}

				options = append(options, providedOptions...)
				options = append(options, ScannerOptionsFromComment(decl.Doc)...)
				g = append(g, NewScanner(options...))
			}
		}
	}

	return g
}

// NewScanner creates a new genieql.Generator from the provided scanner options.
func NewScanner(options ...ScannerOption) genieql.Generator {
	return maybeScanner(newScanner(options...))
}

func maybeScanner(s scanner, err error) genieql.Generator {
	if err != nil {
		return genieql.NewErrGenerator(err)
	}

	return s
}

func newScanner(options ...ScannerOption) (scanner, error) {
	var (
		err error
	)

	// by default enable all modes
	s := scanner{
		scannerConfig{
			Mode: ModeInterface | ModeStatic | ModeDynamic,
		},
	}

	for _, opt := range options {
		if err = opt(&s); err != nil {
			return s, err
		}
	}

	return s, err
}

type scannerConfig struct {
	Context
	Name                  string
	interfaceName         string
	Mode                  mode
	Fields                *ast.FieldList
	ignoreSet             []string
	columnNameTransformer []transform.Transformer
}

type scanner struct {
	scannerConfig
}

func (t scanner) Generate(dst io.Writer) error {
	var (
		err  error
		tmpl *template.Template
	)

	type context struct {
		Name          string
		RowType       string
		InterfaceName string
		Parameters    []*ast.Field
		Columns       []genieql.ColumnMap
	}

	ctx := context{
		RowType:       t.Context.Configuration.RowType,
		Name:          t.Name,
		InterfaceName: stringsx.ToPublic(stringsx.DefaultIfBlank(t.interfaceName, t.Name)),
		Parameters:    t.Fields.List,
	}

	if ctx.Columns, err = ColumnMapFromFields(t.Context, t.Fields.List...); err != nil {
		return errorsx.Wrap(err, "failed to map fields")
	}

	funcMap := template.FuncMap{
		"ast":       astPrint,
		"expr":      types.ExprString,
		"scan":      scan,
		"arguments": argumentsAsPointers,
		"nulltype":  nulltypes(t.Context),
		"typeexpr":  func(x string) ast.Expr { return astutil.MustParseExpr(t.FileSet, x) },
		"decode":    decode(t.Context),
		"error": func() func(string) ast.Node {
			return func(local string) ast.Node {
				return astutil.Return(
					ast.NewIdent(local),
				)
			}
		},
		"title":   stringsx.ToPublic,
		"private": stringsx.ToPrivate,
	}

	if t.Mode.Enabled(ModeInterface) {
		tmpl = template.Must(template.New("interface").Funcs(funcMap).Parse(interfaceScanner))
		if err = tmpl.Execute(dst, ctx); err != nil {
			return errorsx.Wrap(err, "failed to generate interface scanner")
		}

		if _, err = dst.Write([]byte("\n")); err != nil {
			return errorsx.Wrap(err, "failed to write newline")
		}
	}

	if t.Mode.Enabled(ModeStatic) {
		// If column constant is explicitly disabled do not enable it.
		if t.Mode.Disabled(ModeStaticDisableColumns) {
			cc := NewColumnConstantFromFieldList(
				t.Context,
				fmt.Sprintf("%sStaticColumns", stringsx.ToPublic(t.Name)),
				t.Dialect.ColumnNameTransformer(
					t.columnNameTransformer...,
				),
				t.Fields,
			)
			if err = cc.Generate(dst); err != nil {
				return err
			}
		}

		tmpl = template.Must(template.New("static").Funcs(funcMap).Parse(staticScanner))
		if err = tmpl.Execute(dst, ctx); err != nil {
			return errorsx.Wrapf(err, "failed to generate static scanner: '''\n%s\n'''", staticScanner)
		}

		if _, err = dst.Write([]byte("\n")); err != nil {
			return errorsx.Wrap(err, "failed to write newline")
		}

		tmpl = template.Must(template.New("static-row").Funcs(funcMap).Parse(staticRowScanner))
		if err = tmpl.Execute(dst, ctx); err != nil {
			return errorsx.Wrap(err, "failed to generate static row scanner")
		}

		if _, err = dst.Write([]byte("\n")); err != nil {
			return errorsx.Wrap(err, "failed to write newline")
		}
	}

	if t.Mode.Enabled(ModeDynamic) {
		tmpl = template.Must(template.New("dynamic").Funcs(funcMap).Parse(dynamicScanner))
		if err = tmpl.Execute(dst, ctx); err != nil {
			return errorsx.Wrap(err, "failed to generate dynamic scanner")
		}
	}

	return nil
}

// turns an array of column mappings into the inputs to the
// scan function.
func scan(columns []genieql.ColumnMap) string {
	args := []string{}

	for idx, col := range columns {
		args = append(args, fmt.Sprintf("&%s", types.ExprString(col.Local(idx))))
	}

	return strings.Join(args, ", ")
}

const interfaceScanner = `
// {{.InterfaceName}} scanner interface.
type {{.InterfaceName}} interface {
	Scan({{ .Parameters | arguments }}) error
	Next() bool
	Close() error
	Err() error
}

type err{{.InterfaceName}} struct {
	e error
}

func (t err{{.InterfaceName}}) Scan({{ .Parameters | arguments }}) error {
	return t.e
}

func (t err{{.InterfaceName}}) Next() bool {
	return false
}

func (t err{{.InterfaceName}}) Err() error {
	return t.e
}

func (t err{{.InterfaceName}}) Close() error {
	return nil
}
`

const staticScanner = `// New{{.Name | title}}Static creates a scanner that operates on a static
// set of columns that are always returned in the same order.
func New{{.Name | title}}Static(rows *sql.Rows, err error) {{.InterfaceName}} {
	if err != nil {
		return err{{.InterfaceName}}{e: err}
	}

	return {{.Name | private}}Static{
		Rows: rows,
	}
}

// {{.Name | private}}Static generated by genieql
type {{.Name | private}}Static struct {
	Rows *sql.Rows
}

// Scan generated by genieql
func (t {{.Name | private}}Static) Scan({{ .Parameters | arguments }}) error {
	var (
		{{- range $index, $column := .Columns }}
		{{ $column.Local $index }} {{ $column.Definition.ColumnType | typeexpr | expr -}}
		{{ end }}
	)

	if err := t.Rows.Scan({{ .Columns | scan}}); err != nil {
		return err
	}

	{{ range $index, $column := .Columns}}
	{{ range $_, $stmt := decode $index $column error -}}
	{{ $stmt | ast }}
	{{ end }}
	{{ end }}
	return t.Rows.Err()
}

// Err generated by genieql
func (t {{.Name | private}}Static) Err() error {
	return t.Rows.Err()
}

// Close generated by genieql
func (t {{.Name | private}}Static) Close() error {
	if t.Rows == nil {
		return nil
	}
	return t.Rows.Close()
}

// Next generated by genieql
func (t {{.Name | private}}Static) Next() bool {
	return t.Rows.Next()
}
`

const staticRowScanner = `// New{{.Name | title}}StaticRow creates a scanner that operates on a static
// set of columns that are always returned in the same order, only scans a single row.
func New{{.Name | title}}StaticRow(row {{.RowType}}) {{.Name | title}}StaticRow {
	return {{.Name | title}}StaticRow {
		row: row,
	}
}

// {{.Name | title}}StaticRow generated by genieql
type {{.Name | title}}StaticRow struct {
	err error
	row {{.RowType}}
}

// Scan generated by genieql
func (t {{.Name | title}}StaticRow) Scan({{ .Parameters | arguments }}) error {
	var (
		{{- range $index, $column := .Columns }}
		{{ $column.Local $index }} {{ $column.Definition.ColumnType | typeexpr | expr -}}
		{{ end }}
	)

	if t.err != nil {
		return t.err
	}

	if err := t.row.Scan({{ .Columns | scan}}); err != nil {
		return err
	}

	{{ range $index, $column := .Columns}}
	{{ range $_, $stmt := decode $index $column error -}}
	{{ $stmt | ast }}
	{{ end }}
	{{ end }}
	return nil
}

// Err set an error to return by scan
func (t {{.Name | title}}StaticRow) Err(err error) {{.Name | title}}StaticRow {
	t.err = err
	return t
}
`

const dynamicScanner = `
// New{{.Name | title}}Dynamic creates a scanner that operates on a dynamic
// set of columns that can be returned in any subset/order.
func New{{.Name | title}}Dynamic(rows *sql.Rows, err error) {{.InterfaceName}} {
	if err != nil {
		return err{{.InterfaceName}}{e: err}
	}

	return {{.Name | private}}Dynamic{
		Rows: rows,
	}
}

// {{.Name | private}}Dynamic generated by genieql
type {{.Name | private}}Dynamic struct {
	Rows *sql.Rows
}

// Scan generated by genieql
func (t {{.Name | private}}Dynamic) Scan({{ .Parameters | arguments }}) error {
	const (
		{{- range $index, $column := .Columns }}
		cn{{$index}} = "{{$column.Name}}"
		{{- end }}
	)
	var (
		ignored sql.RawBytes
		err     error
		columns []string
		dst     []interface{}
		{{- range $index, $column := .Columns }}
		{{ $column.Local $index }} {{ $column.Definition.ColumnType | typeexpr | expr -}}
		{{ end }}
	)

	if columns, err = t.Rows.Columns(); err != nil {
		return err
	}

	dst = make([]interface{}, 0, len(columns))

	for _, column := range columns {
		switch column {
		{{- range $index, $column := .Columns }}
		case cn{{$index}}:
			dst = append(dst, &{{ $column.Local $index -}})
		{{- end }}
		default:
			dst = append(dst, &ignored)
		}
	}

	if err := t.Rows.Scan(dst...); err != nil {
		return err
	}

	for _, column := range columns {
		switch column {
		{{- range $index, $column := .Columns}}
		case cn{{$index}}:
			{{ range $_, $stmt := decode $index $column error -}}
			{{ $stmt | ast }}
			{{ end -}}
		{{- end }}
		}
	}

	return t.Rows.Err()
}

// Err generated by genieql
func (t {{.Name | private}}Dynamic) Err() error {
	return t.Rows.Err()
}

// Close generated by genieql
func (t {{.Name | private}}Dynamic) Close() error {
	if t.Rows == nil {
		return nil
	}
	return t.Rows.Close()
}

// Next generated by genieql
func (t {{.Name | private}}Dynamic) Next() bool {
	return t.Rows.Next()
}
`
