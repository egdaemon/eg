package ginterp

import (
	"fmt"
	"go/ast"
	"io"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/astcodec"
	"github.com/james-lawrence/genieql/generators"
	"github.com/james-lawrence/genieql/internal/errorsx"
)

// Structure - configuration interface for generating structures.
type Structure interface {
	genieql.Generator // must satisfy the generator interface
	// From generate the structure based on the record definition.
	From(definition) Structure
	Table(string) definition
	Query(string) definition
	// OptionTransformColumns(x ...func(genieql.ColumnInfo) genieql.ColumnInfo) Structure
}

func StructureFromFile(cctx generators.Context, name string, tree *ast.File) (Structure, error) {
	var (
		fn *ast.FuncDecl
	)

	if fn = astcodec.FileFindDecl[*ast.FuncDecl](tree, astcodec.FindFunctionsByName(name)); fn == nil {
		return nil, fmt.Errorf("unable to locate function declaration for scanner: %s", name)
	}

	return NewStructure(
		cctx,
		name,
		fn.Doc,
	), nil
}

// NewStructure instantiate a new structure generator. it uses the name of function
// that calls Define as the name of the emitted type.
func NewStructure(ctx generators.Context, name string, comment *ast.CommentGroup) Structure {
	return &sconfig{ctx: ctx, name: name, comment: comment}
}

type sconfig struct {
	name    string
	comment *ast.CommentGroup
	d       definition
	ctx     generators.Context
}

func (t *sconfig) Generate(dst io.Writer) error {
	if t.d == nil {
		return errorsx.String("missing definition, unable to generate structure. please call the From method")
	}

	t.ctx.Println("generation of", t.name, "initiated")
	defer t.ctx.Println("generation of", t.name, "completed")

	return generators.NewStructure(
		generators.StructOptionContext(t.ctx),
		generators.StructOptionName(t.name),
		generators.StructOptionComment(t.comment),
		generators.StructOptionColumnsStrategy(func(generators.Context) ([]genieql.ColumnInfo, error) {
			return t.d.Columns()
		}),
		generators.StructOptionMappingConfigOptions(
			genieql.MCOPackage(t.ctx.CurrentPackage),
		),
	).Generate(dst)
}

func (t *sconfig) From(d definition) Structure {
	t.d = d
	return t
}

func (t sconfig) Table(s string) definition {
	return Table(t.ctx.Driver, t.ctx.Dialect, s)
}

func (t sconfig) Query(s string) definition {
	return Query(t.ctx.Driver, t.ctx.Dialect, s)
}
