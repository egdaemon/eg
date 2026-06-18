package typespec

import (
	"go/ast"
	"go/format"
	"go/token"
	"io"
)

// compiler consumes a definition and returns a function declaration node.
type compiler interface {
	Compile() (*ast.TypeSpec, error)
}

func NewType(name string, t ast.Expr) GenDecl {
	return GenDecl{
		Name: ast.NewIdent(name),
		Type: t,
	}
}

type GenDecl struct {
	Name *ast.Ident
	Type ast.Expr
}

// Compile using the provided definition.
func (t GenDecl) Compile() (_ *ast.TypeSpec, err error) {
	return &ast.TypeSpec{
			Name: t.Name,
			Type: t.Type,
		},
		nil
}

// CompileInto the provided io.Writer
func CompileInto(dst io.Writer, c compiler) (err error) {
	var (
		n ast.Spec
	)

	if n, err = c.Compile(); err != nil {
		return err
	}

	decl := &ast.GenDecl{
		TokPos: token.Pos(0),
		Tok:    token.TYPE,
		Specs: []ast.Spec{
			n,
		},
	}

	return format.Node(dst, token.NewFileSet(), []ast.Decl{decl})
}
