package astutil

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"strconv"

	"github.com/james-lawrence/genieql/internal/errorsx"
)

// Expr converts a template expression into an ast.Expr node.
func Expr(template string) ast.Expr {
	expr, err := parser.ParseExpr(template)
	errorsx.MaybePanic(err)

	return expr
}

// Field builds an ast.Field from the given type and names.
func Field(typ ast.Expr, names ...*ast.Ident) *ast.Field {
	return &ast.Field{
		Names: names,
		Type:  typ,
	}
}

// Field builds an ast.Field from the given type and names.
func FieldList(els ...*ast.Field) *ast.FieldList {
	return &ast.FieldList{
		List: els,
	}
}

// SelExpr builds an *ast.SelectorExpr.
func SelExpr(lhs, rhs string) *ast.SelectorExpr {
	return &ast.SelectorExpr{
		X:   ast.NewIdent(lhs),
		Sel: ast.NewIdent(rhs),
	}
}

// ExprTemplateList converts a series of template expressions into a slice of
// ast.Expr.
func ExprTemplateList(examples ...string) []ast.Expr {
	result := make([]ast.Expr, 0, len(examples))
	for _, example := range examples {
		result = append(result, Expr(example))
	}
	return result
}

// ExprList converts a series of template expressions into a slice of
// ast.Expr.
func ExprList(in ...ast.Expr) []ast.Expr {
	result := make([]ast.Expr, 0, len(in))
	return append(result, in...)
}

// Return - creates a return statement from the provided expressions.
func Return(expressions ...ast.Expr) ast.Stmt {
	return &ast.ReturnStmt{
		Results: expressions,
	}
}

// Block - creates a block statement from the provided statements.
func Block(statements ...ast.Stmt) *ast.BlockStmt {
	if len(statements) == 0 {
		return &ast.BlockStmt{
			List: statements,
		}
	}

	return &ast.BlockStmt{
		Lbrace: statements[0].Pos(),
		List:   statements,
		Rbrace: statements[len(statements)-1].End(),
	}
}

// If - creates an if statement.
func If(init ast.Stmt, condition ast.Expr, body *ast.BlockStmt, els ast.Stmt) *ast.IfStmt {
	return &ast.IfStmt{
		Init: init,
		Cond: condition,
		Body: body,
		Else: els,
	}
}

// For - creates a for statement
func For(init ast.Stmt, condition ast.Expr, post ast.Stmt, body *ast.BlockStmt) *ast.ForStmt {
	return &ast.ForStmt{
		Init: init,
		Cond: condition,
		Post: post,
		Body: body,
	}
}

// Range - create a range statement loop. for x,y := range {}
func Range(key, value ast.Expr, tok token.Token, iterable ast.Expr, body *ast.BlockStmt) *ast.RangeStmt {
	return &ast.RangeStmt{
		Key:   key,
		Value: value,
		Tok:   tok,
		X:     iterable,
		Body:  body,
	}
}

// Switch - create a switch statement.
func Switch(init ast.Stmt, tag ast.Expr, body *ast.BlockStmt) *ast.SwitchStmt {
	return &ast.SwitchStmt{
		Init: init,
		Tag:  tag,
		Body: body,
	}
}

// CaseClause - create a clause statement.
func CaseClause(expr []ast.Expr, statements ...ast.Stmt) *ast.CaseClause {
	return &ast.CaseClause{
		List: expr,
		Body: statements,
	}
}

// Assign - creates an assignment statement from the provided
// expressions and token.
func Assign(to []ast.Expr, tok token.Token, from []ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{
		Lhs: to,
		Tok: tok,
		Rhs: from,
	}
}

// ValueSpec creates a value spec. i.e) x,y,z int
func ValueSpec(typ ast.Expr, names ...*ast.Ident) *ast.ValueSpec {
	return &ast.ValueSpec{
		Names: names,
		Type:  typ,
	}
}

func DeclStmt(d ast.Decl) *ast.DeclStmt {
	return &ast.DeclStmt{Decl: d}
}

func FuncLiteral(d *ast.FuncDecl) *ast.FuncLit {
	return &ast.FuncLit{Type: d.Type, Body: d.Body}
}

func FuncType(params, results *ast.FieldList) *ast.FuncType {
	return &ast.FuncType{
		Params:  params,
		Results: results,
	}
}

// VarList creates a variable list. i.e) var (a int, b bool, c string)
func VarList(specs ...ast.Spec) *ast.GenDecl {
	var (
		lparen = token.NoPos
		rparen = token.NoPos
	)

	if len(specs) > 0 {
		lparen = specs[0].Pos()
		rparen = specs[len(specs)-1].End()
	}

	return &ast.GenDecl{
		Tok:    token.VAR,
		TokPos: lparen - 1,
		Lparen: lparen,
		Specs:  specs,
		Rparen: rparen,
	}
}

func literalDecl(tok token.Token, name string, x ast.Expr) *ast.GenDecl {
	return &ast.GenDecl{
		Tok: tok,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names: []*ast.Ident{
					{
						Name: name,
						Obj: &ast.Object{
							Kind: ast.Con,
							Name: name,
						},
					},
				},
				Values: []ast.Expr{
					x,
				},
			},
		},
	}
}

// Const creates a constant. i.e) const a = 0
func Const(name string, x ast.Expr) ast.Decl {
	return literalDecl(token.CONST, name, x)
}

// ConstantDecl create a constant declaration statement.
func ConstDecl(q string, qe ast.Expr) *ast.DeclStmt {
	return &ast.DeclStmt{Decl: Const(q, qe)}
}

// CallExpr - creates a function call expression with the provided argument
// expressions.
func CallExpr(fun ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:  fun,
		Args: args,
	}
}

// CallExpr - creates a function call expression with the provided argument
// expressions.
func CallExprEllipsis(fun ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:      fun,
		Args:     args,
		Ellipsis: token.Pos(1),
	}
}

// TransformFields ...
func TransformFields(m func(*ast.Field) *ast.Field, fields ...*ast.Field) []*ast.Field {
	result := make([]*ast.Field, 0, len(fields))
	for _, field := range fields {
		result = append(result, m(field))
	}
	return result
}

// MapFieldsToNameExpr - extracts all the names from the provided fields.
func MapFieldsToNameExpr(args ...*ast.Field) []ast.Expr {
	result := make([]ast.Expr, 0, len(args))
	for _, f := range args {
		result = append(result, MapIdentToExpr(f.Names...)...)
	}
	return result
}

// MapFieldsToValueSpec transform a field to a valuespec
func MapFieldsToValueSpec(args ...*ast.Field) []*ast.ValueSpec {
	result := make([]*ast.ValueSpec, 0, len(args))
	for _, f := range args {
		result = append(result, ValueSpec(f.Type, f.Names...))
	}
	return result
}

// MapValueSpecToSpec transform a field to a valuespec due to weak golang type system
func MapValueSpecToSpec(args ...*ast.ValueSpec) []ast.Spec {
	result := make([]ast.Spec, 0, len(args))
	for _, f := range args {
		result = append(result, f)
	}
	return result
}

// FlattenFields unnests a field with multiple names.
func FlattenFields(args ...*ast.Field) []*ast.Field {
	result := make([]*ast.Field, 0, len(args))
	for _, f := range args {
		for _, name := range f.Names {
			result = append(result, Field(f.Type, name))
		}
	}
	return result
}

// MapFieldsToNameIdent maps the set of fields to their names.
func MapFieldsToNameIdent(args ...*ast.Field) []*ast.Ident {
	result := make([]*ast.Ident, 0, len(args))
	for _, f := range args {
		result = append(result, f.Names...)
	}
	return result
}

// MapFieldsToTypeExpr - extracts the type for each name for each of the provided fields.
// i.e.) a,b int, c string, d float is transformed into: int, int, string, float
func MapFieldsToTypeExpr(args ...*ast.Field) []ast.Expr {
	r := []ast.Expr{}
	for idx, f := range args {
		if len(f.Names) == 0 {
			f.Names = []*ast.Ident{ast.NewIdent(fmt.Sprintf("f%d", idx))}
		}

		for range f.Names {
			r = append(r, f.Type)
		}

	}
	return r
}

// MapIdentToExpr converts all the Ident's to expressions.
func MapIdentToExpr(args ...*ast.Ident) []ast.Expr {
	result := make([]ast.Expr, 0, len(args))

	for _, ident := range args {
		result = append(result, ident)
	}

	return result
}

// MapExprToString maps all the expressions to the corresponding strings.
func MapExprToString(args ...ast.Expr) []string {
	result := make([]string, 0, len(args))
	for _, expr := range args {
		result = append(result, types.ExprString(expr))
	}

	return result
}

// TypePattern build a pattern matcher from the provided expressions.
func TypePattern(pattern ...ast.Expr) func(...ast.Expr) bool {
	return func(testcase ...ast.Expr) bool {
		if len(pattern) != len(testcase) {
			return false
		}

		for idx := range pattern {
			if types.ExprString(pattern[idx]) != types.ExprString(testcase[idx]) {
				return false
			}
		}

		return true
	}
}

// IntegerLiteral builds a integer literal.
func IntegerLiteral(n int) ast.Expr {
	return &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(n)}
}

func BinaryExpr(lhs ast.Expr, op token.Token, rhs ast.Expr) *ast.BinaryExpr {
	return &ast.BinaryExpr{
		X:  lhs,
		Op: op,
		Y:  rhs,
	}
}

// StringLiteral expression
func StringLiteral(s string) ast.Expr {
	return &ast.BasicLit{
		Kind:  token.STRING,
		Value: fmt.Sprintf("`%s`", s),
	}
}

// Print an ast.Node
func Print(n ast.Node) (string, error) {
	if n == nil {
		return "", nil
	}

	dst := bytes.NewBuffer([]byte{})
	fset := token.NewFileSet()
	err := printer.Fprint(dst, fset, n)

	return dst.String(), errorsx.Wrap(err, "failure to print ast")
}

// MustPrint an ast.Node
func MustPrint(n ast.Node) string {
	if s, err := Print(n); err == nil {
		return s
	}
	return ""
}

// StructureFieldSelectors return an array of selector expressions from the given
// idents and a field of fields.
func StructureFieldSelectors(local *ast.Field, fields ...*ast.Field) []ast.Expr {
	selectors := make([]ast.Expr, 0, len(fields))
	for _, n := range local.Names {
		for _, field := range fields {
			sel := MapFieldsToNameIdent(field)[0]
			sel.NamePos = 0
			selectors = append(selectors, &ast.SelectorExpr{
				X:   n,
				Sel: sel,
			})
		}
	}

	return selectors
}

// MustParseExpr panics if the string cannot be parsed into an expression.
func MustParseExpr(fs *token.FileSet, in string) ast.Expr {
	expr, err := parser.ParseExprFrom(fs, "", []byte(in), 0)
	if err != nil {
		panic(errorsx.Wrapf(err, "failed to parse (%s)", in))
	}

	return expr
}

func DereferencedIdent(x ast.Expr) ast.Expr {
	switch real := x.(type) {
	case *ast.StarExpr:
		// log.Printf("localIdent - star: %T - %s\n", real.X, types.ExprString(real.X))
		return real.X
	default:
		// log.Printf("localIdent: %T - %s\n", real, types.ExprString(real))
		return real
	}
}

// dereference types
func Dereference(x ast.Expr) ast.Expr {
	x = UnwrapExpr(x)
	switch x := x.(type) {
	case *ast.SelectorExpr:
		return x
	default:
		// log.Printf("autodereference: %T - %s\n", x, types.ExprString(x))
		return &ast.UnaryExpr{Op: token.MUL, X: x}
	}
}

func UnwrapExpr(x ast.Expr) ast.Expr {
	switch real := x.(type) {
	case *ast.Ellipsis:
		return real.Elt
	case *ast.StarExpr:
		return real.X
	default:
		return x
	}
}
