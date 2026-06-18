package generators

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"log"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/davecgh/go-spew/spew"
	"github.com/serenize/snaker"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/internal/debugx"
	"github.com/james-lawrence/genieql/internal/drivers"
	"github.com/james-lawrence/genieql/internal/errorsx"
)

func genFunctionLiteral(ctx Context, example string, tctx interface{}, errorHandler func(string) ast.Node) (output *ast.FuncLit, err error) {
	var (
		ok     bool
		parsed ast.Node
		buf    bytes.Buffer
		m      = template.FuncMap{
			"ast":  astutil.Print,
			"expr": types.ExprString,
			"debugexpr": func(x ast.Expr) ast.Expr {
				log.Println("debugexpt", types.ExprString(x))
				return x
			},
			"localident":      astutil.DereferencedIdent,
			"autodereference": astutil.Dereference,
			"autoreference":   autoreference,
			"error":           errorHandler,
		}
	)

	if err = template.Must(template.New("genFunctionLiteral").Funcs(m).Parse(example)).Execute(&buf, tctx); err != nil {
		return nil, errorsx.Wrapf(err, "failed to generate from template: '''\n%s\n'''\n%s", example, spew.Sdump(tctx))
	}

	if parsed, err = parser.ParseExprFrom(ctx.FileSet, "", buf.Bytes(), 0); err != nil {
		return nil, errorsx.Wrapf(err, "failed to parse function expression: %s", buf.String())
	}

	if output, ok = parsed.(*ast.FuncLit); !ok {
		return nil, errorsx.Errorf("parsed template expected to result in *ast.FuncLit not %T: %s", example, parsed)
	}

	return output, nil
}

type transforms func(x ast.Expr) ast.Expr

func argumentsNative(ctx Context) transforms {
	def := composeTypeDefinitionsExpr(ctx.Driver.LookupType, drivers.DefaultTypeDefinitions)
	return func(x ast.Expr) (out ast.Expr) {
		var (
			err error
			d   genieql.ColumnDefinition
		)

		if d, err = def(x); err != nil {
			// this is expected.
			return x
		}

		if out, err = parser.ParseExpr(d.Native); err != nil {
			log.Println("failed to parse expression from type definition", err, spew.Sdump(d))
			return x
		}

		debugx.Println("TRANSFORMING", types.ExprString(x), "->", types.ExprString(out))
		return out
	}
}

func nulltypes(ctx Context) transforms {
	typeDefinitions := composeTypeDefinitionsExpr(ctx.Driver.LookupType, drivers.DefaultTypeDefinitions)
	return func(e ast.Expr) (expr ast.Expr) {
		var (
			err error
			d   genieql.ColumnDefinition
		)

		e = removeEllipsis(e)
		if d, err = typeDefinitions(e); err != nil {
			log.Println("failed to locate type definition:", types.ExprString(e))
			return e
		}

		if expr, err = parser.ParseExpr(d.ColumnType); err != nil {
			log.Println("failed to parse expression:", types.ExprString(e), "->", d.ColumnType)
			return e
		}

		return expr
	}
}

// decode a column to a local variable.
func decode(ctx Context) func(int, genieql.ColumnMap, func(string) ast.Node) ([]ast.Stmt, error) {
	lookupTypeDefinition := composeTypeDefinitions(ctx.Driver.LookupType, drivers.DefaultTypeDefinitions)
	return func(i int, column genieql.ColumnMap, errHandler func(string) ast.Node) (output []ast.Stmt, err error) {
		type stmtCtx struct {
			From   ast.Expr
			To     ast.Expr
			Type   ast.Expr
			Column genieql.ColumnMap
		}

		var (
			local = column.Local(i)
			gen   *ast.FuncLit
		)

		if column.Definition.Decode == "" {
			if column.Definition, err = lookupTypeDefinition(column.Definition.Type); err != nil {
				return nil, errorsx.Wrapf(err, "invalid type definition: %s", spew.Sdump(column.Definition))
			}
		}

		typex := astutil.MustParseExpr(ctx.FileSet, column.Definition.Native)
		to := column.Dst
		if column.Definition.Nullable {
			to = &ast.StarExpr{X: astutil.UnwrapExpr(to)}
		}

		if gen, err = genFunctionLiteral(ctx, column.Definition.Decode, stmtCtx{Type: astutil.UnwrapExpr(typex), From: local, To: to, Column: column}, errHandler); err != nil {
			return nil, err
		}

		return gen.Body.List, nil
	}
}

func fallbackDefinition(s string) genieql.ColumnDefinition {
	return genieql.ColumnDefinition{
		Type:       s,
		Native:     s,
		ColumnType: s,
	}
}

// ColumnMapEncoder a column to a local variable.
func ColumnMapEncoder(ctx Context) func(int, genieql.ColumnMap, func(string) ast.Node) ([]ast.Stmt, error) {
	lookupTypeDefinition := composeTypeDefinitions(ctx.Driver.LookupType, drivers.DefaultTypeDefinitions)
	return func(i int, column genieql.ColumnMap, errHandler func(string) ast.Node) (output []ast.Stmt, err error) {
		type stmtCtx struct {
			From   ast.Expr
			To     ast.Expr
			Type   ast.Expr
			Column genieql.ColumnMap
		}

		var (
			local = column.Local(i)
			gen   *ast.FuncLit
		)

		if column.Definition.Encode == "" {
			if d, err := lookupTypeDefinition(column.Definition.Type); err == nil {
				column.Definition = d
			} else {
				column.Definition = fallbackDefinition(column.Definition.Type)
			}
		}

		if column.Definition.Encode == "" {
			log.Printf("skipping %s (%s -> %s) missing encode block\n", column.Name, column.Definition.Type, column.Definition.ColumnType)
			return nil, nil
		}

		// if the mapping represents a variable that is a standalone DB native type then there is no encoding to do.
		if _, ok := column.Dst.(*ast.Ident); ok && column.Definition.Type == column.Definition.ColumnType {
			return nil, nil
		}

		typex := astutil.MustParseExpr(ctx.FileSet, column.Definition.Native)
		from := astutil.UnwrapExpr(column.Dst)
		if column.Definition.Nullable {
			from = &ast.StarExpr{X: from}
		}

		if gen, err = genFunctionLiteral(ctx, column.Definition.Encode, stmtCtx{Type: astutil.UnwrapExpr(typex), From: from, To: local, Column: column}, errHandler); err != nil {
			return nil, err
		}

		return gen.Body.List, nil
	}
}

func argumentsTransform(t transforms) func(fields []*ast.Field) string {
	return func(fields []*ast.Field) string {
		return _arguments(t, fields)
	}
}

func argumentsAsPointers(fields []*ast.Field) string {
	xtransformer := func(x ast.Expr) ast.Expr {
		return &ast.StarExpr{X: x}
	}
	return _arguments(xtransformer, fields)
}

func _arguments(xtransformer func(ast.Expr) ast.Expr, fields []*ast.Field) string {
	result := []string{}
	for _, field := range fields {
		result = append(result,
			strings.Join(
				astutil.MapExprToString(astutil.MapIdentToExpr(field.Names...)...),
				", ",
			)+" "+types.ExprString(xtransformer(field.Type)))
	}
	return strings.Join(result, ", ")
}

// SanitizeFieldIdents transforms the idents of fields to prevent collisions.
func SanitizeFieldIdents(trans func(*ast.Ident) *ast.Ident, fields ...*ast.Field) []*ast.Field {
	normalizeIdent := func(idents []*ast.Ident) []*ast.Ident {
		result := make([]*ast.Ident, 0, len(idents))
		for _, ident := range idents {
			result = append(result, trans(ident))
		}
		return result
	}

	return astutil.TransformFields(func(field *ast.Field) *ast.Field {
		return astutil.Field(field.Type, normalizeIdent(field.Names)...)
	}, fields...)
}

// NormalizeFieldNames normalizes the names of the field.
func NormalizeFieldNames(fields ...*ast.Field) []*ast.Field {
	return normalizeFieldNames(fields...)
}

// normalizes the names of the field.
func normalizeFieldNames(fields ...*ast.Field) []*ast.Field {
	return astutil.TransformFields(func(field *ast.Field) *ast.Field {
		return astutil.Field(field.Type, normalizeIdent(field.Names)...)
	}, fields...)
}

// NormalizeIdent ensures ident obey the following:
// 1. are snakecased.
// 2. are not reserved keywords.
func NormalizeIdent(idents ...*ast.Ident) []*ast.Ident {
	return normalizeIdent(idents)
}

// normalize's the idents.
func normalizeIdent(idents []*ast.Ident) []*ast.Ident {
	result := make([]*ast.Ident, 0, len(idents))

	for _, ident := range idents {
		n := ident.Name

		if !strings.Contains(n, "_") {
			n = snaker.CamelToSnake(ident.Name)
		}

		n = toPrivate(n)

		if reserved(n) {
			n = "_" + n
		}

		result = append(result, ast.NewIdent(n))
	}

	return result
}

func toPrivate(s string) string {
	// ignore strings that start with an _
	s = strings.TrimPrefix(s, "_")

	parts := strings.SplitN(s, "_", 2)
	switch len(parts) {
	case 2:
		return strings.ToLower(parts[0]) + snaker.SnakeToCamel(strings.ToLower(parts[1]))
	default:
		return strings.ToLower(s)
	}
}

func astPrint(n ast.Node) (string, error) {
	if n == nil {
		return "", nil
	}

	dst := bytes.NewBuffer([]byte{})
	fset := token.NewFileSet()
	err := printer.Fprint(dst, fset, n)

	return dst.String(), errorsx.Wrap(err, "failure to print ast")
}

// AllBuiltinTypes returns true iff all the types are builtin to the go runtime.
func AllBuiltinTypes(xs ...ast.Expr) bool {
	return allBuiltinTypes(xs...)
}

func allBuiltinTypes(xs ...ast.Expr) bool {
	for _, x := range xs {
		if !builtinType(x) {
			return false
		}
	}

	return true
}

func builtinType(x ast.Expr) bool {
	name := strings.ReplaceAll(types.ExprString(x), "*", "")
	debugx.Printf("builtinType check %T - %s", x, name)

	for _, t := range types.Typ {
		if name == t.Name() {
			return true
		}
	}

	// TODO these shouldn't really be here. think of a way to associate with the driver.
	switch name {
	case "interface{}":
		fallthrough
	case "time.Time", "[]time.Time", "time.Duration", "[]time.Duration":
		fallthrough
	case "json.RawMessage":
		fallthrough
	case "net.IPNet", "[]net.IPNet":
		fallthrough
	case "net.IP", "[]net.IP":
		fallthrough
	case "net.HardwareAddr":
		fallthrough
	case "[]byte", "[]int", "[]string":
		return true
	default:
		return false
	}
}

// builtinParam converts a *ast.Field that represents a builtin type
// (time.Time,int,float,bool, etc) into an array of ColumnMap.
func builtinParam(ctx Context, param *ast.Field) ([]genieql.ColumnMap, error) {
	columns := make([]genieql.ColumnMap, 0, len(param.Names))
	for _, name := range param.Names {
		typex := types.ExprString(removeEllipsis(param.Type))
		typed, err := ctx.Driver.LookupType(typex)
		if err != nil {
			typed = fallbackDefinition(typex)
		}

		columns = append(columns, genieql.ColumnMap{
			ColumnInfo: genieql.ColumnInfo{
				Name:       name.Name,
				Definition: typed,
			},
			Dst: &ast.StarExpr{X: name},
		})
	}

	return columns, nil
}

func autoreference(x ast.Expr) ast.Expr {
	x = astutil.UnwrapExpr(x)
	switch x := x.(type) {
	case *ast.SelectorExpr:
		return &ast.UnaryExpr{Op: token.AND, X: x}
	}
	return x
}

func determineType(x ast.Expr) ast.Expr {
	switch x := x.(type) {
	case *ast.SelectorExpr:
		return x.Sel
	case *ast.StarExpr:
		return x.X
	default:
		debugx.Printf("determineType: %T - %s", x, types.ExprString(x))
		return x
	}
}

func importPath(ctx Context, x ast.Expr) (string, error) {
	switch x := x.(type) {
	case *ast.SelectorExpr:
		importSelector := func(is *ast.ImportSpec) string {
			if is.Name == nil {
				return filepath.Base(strings.Trim(is.Path.Value, "\""))
			}
			return is.Name.Name
		}

		if src, err := parser.ParseFile(ctx.FileSet, ctx.FileSet.File(x.Pos()).Name(), nil, parser.ImportsOnly); err != nil {
			return "", errorsx.Wrap(err, "failed to read the source file while determining import")
		} else {
			for _, imp := range src.Imports {
				if importSelector(imp) == types.ExprString(x.X) {
					return strings.Trim(imp.Path.Value, "\""), nil
				}
			}

			return "", errorsx.Errorf("failed to match selector with import: %s", types.ExprString(x))
		}
	default:
		return ctx.CurrentPackage.ImportPath, nil
	}
}
