package transpile

import (
	"go/ast"
	"go/token"
	"go/types"
	"log"
)

func grapher(fset *token.FileSet) ast.Visitor {
	return nodePrinter{
		fset: fset,
	}
}

type nodePrinter struct {
	fset *token.FileSet
}

func (t nodePrinter) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return t
	}

	pos := t.fset.PositionFor(node.Pos(), true)
	switch x := node.(type) {
	case *ast.CallExpr:
		log.Println("invocation of", types.ExprString(x.Fun), pos)
	default:
	}
	return t
}
