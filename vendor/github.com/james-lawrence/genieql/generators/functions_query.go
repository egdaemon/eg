package generators

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"strings"
	"text/template"

	"github.com/zieckey/goini"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/internal/errorsx"
)

const defaultQueryParamName = "q"

// QueryFunctionOption options for building query functions.
type QueryFunctionOption func(*queryFunction)

// QFONoop do nothing
func QFONoop(*queryFunction) {}

// QFOName specify the name of the query function.
func QFOName(n string) QueryFunctionOption {
	return func(qf *queryFunction) {
		qf.Name = n
	}
}

// QFOScanner specify the scanner of the query function
func QFOScanner(n *ast.FuncDecl) QueryFunctionOption {
	return func(qf *queryFunction) {
		qf.ScannerDecl = n
	}
}

// QFOBuiltinQueryFromString force the query function to only execute the specified
// query.
func QFOBuiltinQueryFromString(q string) QueryFunctionOption {
	return QFOBuiltinQuery(&ast.BasicLit{
		Kind:  token.STRING,
		Value: fmt.Sprintf("`%s`", q),
	})
}

// QFOBuiltinQuery force the query function to only execute the specified
// query.
func QFOBuiltinQuery(x ast.Expr) QueryFunctionOption {
	return func(qf *queryFunction) {
		qf.BuiltinQuery = x
	}
}

// QFOQueryer the name/type used to execute queries.
func QFOQueryer(name string, x ast.Expr) QueryFunctionOption {
	return func(qf *queryFunction) {
		qf.Queryer = x
		qf.QueryerName = name
	}
}

// QFOQueryerFunction the function to invoke on the Queryer.
func QFOQueryerFunction(x *ast.Ident) QueryFunctionOption {
	return func(qf *queryFunction) {
		qf.QueryerFunction = x
	}
}

// QFOParameters specify the parameters for the query function.
func QFOParameters(p []*ast.Field, qp []ast.Expr) QueryFunctionOption {
	p = normalizeFieldNames(p...)
	return func(qf *queryFunction) {
		qf.Parameters = p
		qf.QueryParameters = qp
	}
}

// QFOSharedParameters - alternate to QFOSharedParameters, use when the all params to the function
// are also the params to the query.
func QFOSharedParameters(params ...*ast.Field) QueryFunctionOption {
	params = normalizeFieldNames(params...)
	return func(qf *queryFunction) {
		qf.Parameters = params
		qf.QueryParameters = astutil.MapFieldsToNameExpr(params...)
	}
}

// QFOTemplate sets the template for the query function
func QFOTemplate(tmpl *template.Template) QueryFunctionOption {
	return func(qf *queryFunction) {
		qf.Template = tmpl
	}
}

// QFOComment set the comment for the generated function.
func QFOComment(c *ast.CommentGroup) QueryFunctionOption {
	return func(qf *queryFunction) {
		qf.Comment = c
	}
}

// QFOIgnore set the names of the fields to ignore.
func QFOIgnore(ignore ...string) QueryFunctionOption {
	return func(qf *queryFunction) {
		qf.Ignore = ignore
	}
}

// QFOFromComment extracts options from a ast.CommentGroup.
func QFOFromComment(comments *ast.CommentGroup) ([]string, []QueryFunctionOption, error) {
	var (
		err       error
		options   []QueryFunctionOption
		defaulted []string
		ini       *goini.INI
	)

	if comments == nil {
		return defaulted, options, err
	}

	if ini, err = ParseCommentOptions(comments); err != nil {
		return defaulted, options, err
	}

	if x, ok := CommentOptionQuery(ini); ok {
		options = append(options, QFOBuiltinQuery(x))
	}

	defaulted, _ = CommentOptionDefaultColumns(ini)

	return defaulted, options, nil
}

func maybeQFO(ctx Context, options []QueryFunctionOption, err error) genieql.Generator {
	if err != nil {
		return genieql.NewErrGenerator(err)
	}

	return NewQueryFunction(ctx, options...)
}

func generatorFromFuncType(ctx Context, name *ast.Ident, comment *ast.CommentGroup, ft *ast.FuncType, poptions ...QueryFunctionOption) (Context, []QueryFunctionOption, error) {
	var (
		err             error
		defaulted       []string
		nameOpt         QueryFunctionOption
		queryer, params QueryFunctionOption
		commentOptions  []QueryFunctionOption
		scannerOption   QueryFunctionOption
	)

	// validations
	if ft.Params.NumFields() < 1 {
		return ctx, []QueryFunctionOption(nil), errorsx.Errorf("function prototype (%s) requires at least the type which will perform the query. i.e.) *sql.DB", name)
	}

	if ft.Results.NumFields() != 1 {
		return ctx, []QueryFunctionOption(nil), errorsx.Errorf("function prototype (%s) requires a single function as the return value", name)
	}

	if defaulted, commentOptions, err = QFOFromComment(comment); err != nil {
		return ctx, []QueryFunctionOption(nil), errorsx.Errorf("function prototype (%s) comment options are invalid", name)
	}

	if queryer, params, err = extractOptionsFromParams(ctx, defaulted, ft.Params.List...); err != nil {
		return ctx, []QueryFunctionOption(nil), errorsx.Wrapf(err, "function prototype (%s) parameters are invalid", name)
	}

	if scannerOption, err = extractOptionsFromResult(ctx, ft.Results.List[0]); err != nil {
		return ctx, []QueryFunctionOption(nil), errorsx.Wrapf(err, "function prototype (%s) scanner option is invalid", name)
	}

	nameOpt = QFONoop
	if name != nil {
		nameOpt = QFOName(name.Name)
	}

	options := append(
		poptions,
		nameOpt,
		queryer,
		params,
		scannerOption,
	)

	return ctx, append(options, commentOptions...), nil
}

// NewQueryFunctionFromGenDecl creates a function generator from the provided *ast.GenDecl
func NewQueryFunctionFromGenDecl(ctx Context, decl *ast.GenDecl, options ...QueryFunctionOption) []genieql.Generator {
	g := make([]genieql.Generator, 0, len(decl.Specs))
	for _, spec := range decl.Specs {
		if ts, ok := spec.(*ast.TypeSpec); ok {
			if ft, ok := ts.Type.(*ast.FuncType); ok {
				g = append(g, maybeQFO(generatorFromFuncType(ctx, ts.Name, decl.Doc, ft, options...)))
			}
		}
	}

	return g
}

// NewQueryFunctionFromFuncDecl creates a function generator from the provided *ast.GenDecl
func NewQueryFunctionFromFuncDecl(ctx Context, decl *ast.FuncDecl, options ...QueryFunctionOption) genieql.Generator {
	options = append(options, extractOptionsFromFunctionDecls(decl.Body)...)
	return maybeQFO(generatorFromFuncType(ctx, decl.Name, decl.Doc, decl.Type, options...))
}

// NewQueryFunctionFromFuncType okay wat
func NewQueryFunctionFromFuncType(ctx Context, node *ast.FuncType, options ...QueryFunctionOption) genieql.Generator {
	return maybeQFO(generatorFromFuncType(ctx, nil, nil, node, options...))
}

func extractOptionsFromFunctionDecls(body *ast.BlockStmt) []QueryFunctionOption {
	options := []QueryFunctionOption{}

	for _, val := range genieql.SelectValues(body) {
		switch val.Ident.Name {
		case "query":
			options = append(options, QFOBuiltinQuery(val.Value))
		}
	}

	return options
}

func extractOptionsFromParams(ctx Context, defaultedSet []string, fields ...*ast.Field) (QueryFunctionOption, QueryFunctionOption, error) {
	var (
		err          error
		params       []*ast.Field
		queryParams  []ast.Expr
		mQueryParams []*ast.Field
	)
	queryerf, paramsf := fields[0], fields[1:]

	for _, param := range paramsf {
		param = normalizeFieldNames(param)[0]
		if builtinType(astutil.UnwrapExpr(param.Type)) {
			params = append(params, param)
			queryParams = append(queryParams, astutil.MapFieldsToNameExpr(param)...)
		} else {
			if _, mQueryParams, err = mappedStructure(ctx, param, defaultedSet...); err != nil {
				return nil, nil, err
			}
			params = append(params, param)
			queryParams = append(queryParams, structureQueryParameters(param, mQueryParams...)...)
		}
	}

	return QFOQueryer(defaultQueryParamName, queryerf.Type), QFOParameters(params, queryParams), nil
}

func extractOptionsFromResult(ctx Context, field *ast.Field) (QueryFunctionOption, error) {
	util := genieql.NewSearcher(ctx.FileSet, ctx.CurrentPackage)
	scanner, err := util.FindFunction(func(s string) bool {
		return s == types.ExprString(field.Type)
	})

	return QFOScanner(scanner), err
}

// NewQueryFunction build a query function generator from the provided options.
func NewQueryFunction(ctx Context, options ...QueryFunctionOption) genieql.Generator {
	qf := queryFunction{
		Context: ctx,
		Comment: &ast.CommentGroup{
			List: []*ast.Comment{},
		},
		Template:    defaultQueryFuncTemplate(ctx),
		Parameters:  []*ast.Field{},
		QueryerName: defaultQueryParamName,
		Queryer:     &ast.StarExpr{X: &ast.SelectorExpr{X: ast.NewIdent("sql"), Sel: ast.NewIdent("DB")}},
	}

	qf.Apply(options...)

	pattern := astutil.MapFieldsToTypeExpr(qf.ScannerDecl.Type.Params.List...)

	// attempt to infer the type from the pattern of the scanner function.
	if qf.QueryerFunction != nil {
		// nothing to do here.
	} else if queryPattern(pattern...) {
		qf.QueryerFunction = ast.NewIdent("Query")
		qf.ErrHandler = func(local string) ast.Node {
			return astutil.Return(
				astutil.CallExpr(qf.ScannerDecl.Name, ast.NewIdent("nil"), ast.NewIdent(local)),
			)
		}
	} else if queryRowPattern(pattern...) {
		qf.QueryerFunction = ast.NewIdent("QueryRow")
		qf.ErrHandler = func(local string) ast.Node {
			return astutil.Return(
				astutil.CallExpr(
					&ast.SelectorExpr{
						X:   astutil.CallExpr(qf.ScannerDecl.Name, ast.NewIdent("nil")),
						Sel: ast.NewIdent("Err"),
					},
					ast.NewIdent(local),
				),
			)
		}
	} else {
		return genieql.NewErrGenerator(errorsx.Errorf("a query function was not provided and failed to infer from the scanner function parameter list"))
	}

	return qf
}

type queryFunction struct {
	Context
	Name            string
	ScannerDecl     *ast.FuncDecl
	BuiltinQuery    ast.Expr
	Queryer         ast.Expr
	QueryerName     string
	QueryerFunction *ast.Ident
	QueryParameters []ast.Expr
	Parameters      []*ast.Field
	Ignore          []string
	Template        *template.Template
	Comment         *ast.CommentGroup
	ErrHandler      func(string) ast.Node
}

func (t *queryFunction) Apply(options ...QueryFunctionOption) *queryFunction {
	for _, opt := range options {
		opt(t)
	}
	return t
}

func (t queryFunction) Generate(dst io.Writer) (err error) {
	type context struct {
		Name            string
		ScannerFunc     ast.Expr
		ScannerType     ast.Expr
		BuiltinQuery    ast.Node
		Exploders       []ast.Stmt
		Queryer         ast.Expr
		Parameters      []*ast.Field
		QueryParameters []ast.Expr
		Comment         string
		Columns         []genieql.ColumnMap
		Error           func(string) ast.Node
	}

	const (
		defaultQueryName = "query"
	)

	var (
		columns         []genieql.ColumnMap
		parameters      []*ast.Field
		queryParameters []ast.Expr
		query           *ast.CallExpr
		queryParam      = ast.NewIdent(defaultQueryName)
	)

	qliteral := func(name string, x ast.Expr) ast.Decl {
		if x == nil {
			return nil
		}

		switch x.(type) {
		case *ast.BasicLit:
			return genieql.QueryLiteral2(token.CONST, name, x)
		default:
			return genieql.QueryLiteral2(token.VAR, name, x)
		}
	}

	// if any of the parameters have the same name as the queryParam use a fallback.
	for _, p := range astutil.MapExprToString(t.QueryParameters...) {
		if p == defaultQueryName {
			queryParam = ast.NewIdent(fmt.Sprintf("%s%s", t.Name, strings.Title(defaultQueryName)))
			break
		}
	}

	// log.Println("mapping fields", strings.Join(astutil.MapExprToString(astutil.MapFieldsToTypeExpr(t.Parameters...)...), ","))
	if columns, err = MapFields(t.Context, t.Parameters, t.Ignore...); err != nil {
		return errorsx.Wrap(err, "failed to map fields")
	}
	// log.Println("parameters", len(t.Parameters), len(t.QueryParameters), len(columns))
	// log.Println("mapping fields")

	parameters = buildParameters(
		t.BuiltinQuery == nil,
		astutil.Field(t.Queryer, ast.NewIdent(t.QueryerName)),
		astutil.Field(ast.NewIdent("string"), queryParam),
		t.Parameters...,
	)

	cToP := func(columns []genieql.ColumnMap) (result []ast.Expr) {
		for i, c := range columns {
			if c.Definition.ColumnType == c.Definition.Native {
				result = append(result, astutil.UnwrapExpr(c.Dst))
				continue
			}
			result = append(result, c.Local(i))
		}
		return result
	}

	queryParameters = buildQueryParameters(astutil.Field(ast.NewIdent("string"), queryParam), cToP(columns)...)

	// if we're dealing with an ellipsis parameter function
	// mark the CallExpr Ellipsis
	// this should only be the case when t.Parameters ends with
	// an ast.Ellipsis expression.
	// this allows for the creation of a generic function:
	// func F(q sql.DB, query, params ...interface{}) StaticExampleScanner
	query = &ast.CallExpr{
		Fun:      &ast.SelectorExpr{X: ast.NewIdent(t.QueryerName), Sel: t.QueryerFunction},
		Args:     queryParameters,
		Ellipsis: isEllipsis(t.Parameters),
	}

	ctx := context{
		Name:         t.Name,
		Comment:      t.Comment.Text(),
		ScannerType:  t.ScannerDecl.Type.Results.List[0].Type,
		ScannerFunc:  t.ScannerDecl.Name,
		BuiltinQuery: qliteral(queryParam.Name, t.BuiltinQuery),
		Parameters:   parameters,
		Queryer:      query,
		Columns:      columns,
		Error:        t.ErrHandler,
	}

	return errorsx.Wrap(t.Template.Execute(dst, ctx), "failed to generate query function")
}

func buildParameters(queryInParams bool, queryer, query *ast.Field, params ...*ast.Field) []*ast.Field {
	var (
		parameters []*ast.Field
	)

	params = normalizeFieldNames(params...)
	// [] -> [q sqlx.Queryer]
	parameters = append(parameters, queryer)
	// [q sqlx.Queryer] -> [q sqlx.Queryer, query string]
	if queryInParams {
		parameters = append(parameters, query)
	}
	// [q sqlx.Queryer, query string] -> [q sqlx.Queryer, query string, param1 int, param2 bool]
	parameters = append(parameters, params...)

	return parameters
}

func buildQueryParameters(query *ast.Field, params ...ast.Expr) []ast.Expr {
	return append(astutil.MapFieldsToNameExpr(query), params...)
}

func defaultQueryFuncTemplate(ctx Context) *template.Template {
	const defaultQueryFunc = `// {{.Name}} generated by genieql
		func {{.Name}}({{ .Parameters | arguments }}) {{ .ScannerType | expr }} {
			{{- $filtered := .Columns | removeidenticaltypes }}
			{{- $root := . }}
			{{- .BuiltinQuery | ast }}
			{{- if $filtered }}
			var (
				{{- range $index, $column := $filtered }}
				{{ $column.Local $index }} {{ $column.Definition.ColumnType | typeexpr | expr -}}
				{{ end }}
			)
			{{ range $index, $column := $filtered}}
			{{ range $_, $stmt := encode $index $column $root.Error -}}
			{{ $stmt | ast }}
			{{ end }}
			{{ end }}
			{{ end }}
			return {{ .ScannerFunc | expr }}({{ .Queryer | expr }})
		}
	`
	var (
		defaultQueryFuncMap = template.FuncMap{
			"removeidenticaltypes": func(columns []genieql.ColumnMap) (results []genieql.ColumnMap) {
				for _, c := range columns {
					if c.Definition.ColumnType == c.Definition.Native {
						continue
					}

					results = append(results, c)
				}

				return results
			},
			"expr":      types.ExprString,
			"arguments": argumentsTransform(argumentsNative(ctx)),
			"encode":    ColumnMapEncoder(ctx),
			"ast":       astPrint,
			"nulltype":  nulltypes(ctx),
			"typeexpr":  func(x string) ast.Expr { return astutil.MustParseExpr(ctx.FileSet, x) },
		}
		defaultQueryFuncTemplate = template.Must(template.New("query-function").Funcs(defaultQueryFuncMap).Parse(defaultQueryFunc))
	)

	return defaultQueryFuncTemplate
}

func isEllipsis(fields []*ast.Field) token.Pos {
	var (
		x ast.Expr
	)

	if len(fields) == 0 {
		return token.Pos(0)
	}

	x = fields[len(fields)-1].Type

	if _, isEllipsis := x.(*ast.Ellipsis); !isEllipsis {
		return token.Pos(0)
	}

	return token.Pos(1)
}
