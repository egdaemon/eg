package astbuild

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// Expr converts a template expression into an ast.Expr node.
func Expr(template string) ast.Expr {
	expr, err := parser.ParseExpr(template)
	if err != nil {
		panic(err)
	}

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
