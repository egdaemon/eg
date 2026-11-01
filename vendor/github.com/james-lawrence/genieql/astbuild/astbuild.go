package astbuild

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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

func CallExpr(fun ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{
		Fun:  fun,
		Args: args,
	}
}

func SelExpr(lhs, rhs string) *ast.SelectorExpr {
	return &ast.SelectorExpr{
		X:   ast.NewIdent(lhs),
		Sel: ast.NewIdent(rhs),
	}
}

// IntegerLiteral builds a integer literal.
func IntegerLiteral(n int) *ast.BasicLit {
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
func StringLiteral(s string) *ast.BasicLit {
	return &ast.BasicLit{
		Kind:  token.STRING,
		Value: fmt.Sprintf("`%s`", s),
	}
}

func StringQuotedLiteral(s string) *ast.BasicLit {
	return &ast.BasicLit{
		Kind:  token.STRING,
		Value: fmt.Sprintf("\"%s\"", s),
	}
}

func ImportSpecLiteral(name *ast.Ident, path string) *ast.ImportSpec {
	return &ast.ImportSpec{
		Name: name,
		Path: StringQuotedLiteral(path),
	}
}

func FnBody(a *ast.FuncDecl) *ast.BlockStmt {
	if a == nil || a.Body == nil {
		return &ast.BlockStmt{}
	}

	return a.Body
}

// GenDeclToDecl upcases GenDecl to Decl.
func GenDeclToDecl(decls ...*ast.GenDecl) (results []ast.Decl) {
	for _, d := range decls {
		results = append(results, d)
	}
	return results
}
