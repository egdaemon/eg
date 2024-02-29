package transpile

import (
	"go/ast"
	"go/types"

	"github.com/egdaemon/eg/astbuild"
	"github.com/egdaemon/eg/astcodec"
)

func replaceRef(yakident string, refexpr *ast.SelectorExpr) ast.Visitor {
	return astcodec.NewCallExprReplacement(func(ce *ast.CallExpr) *ast.CallExpr {
		args := []ast.Expr{
			astbuild.StringLiteral(types.ExprString(ce.Args[0])),
		}
		args = append(args, ce.Args...)
		return astbuild.CallExpr(astbuild.SelExpr(yakident, "UnsafeTranspiledRef"), args...)
	}, func(ce *ast.CallExpr) bool {
		return astcodec.TypePattern(refexpr)(ce.Fun)
	})
}
