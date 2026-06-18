package ginterp

import (
	"fmt"
	"go/ast"
	"go/printer"
	"io"
	"log"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/astcodec"
	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/generators"
	"github.com/james-lawrence/genieql/generators/functions"
	"github.com/james-lawrence/genieql/internal/errorsx"
)

// Function configuration interface for generating functions.
type Function interface {
	genieql.Generator // must satisfy the generator interface
	Query(string) Function
}

// NewFunction instantiate a new function generator. it uses the name of function
// that calls Define as the name of the generated function.
func NewFunction(
	ctx generators.Context,
	name string,
	signature *ast.FuncType,
	comment *ast.CommentGroup,
) Function {
	return &function{
		ctx:       ctx,
		name:      name,
		signature: signature,
		comment:   comment,
	}
}

func FunctionFromFile(cctx generators.Context, name string, tree *ast.File) (Function, error) {
	var (
		ok          bool
		pos         *ast.FuncDecl
		scanner     *ast.FuncDecl // scanner to use for the results.
		declPattern *ast.FuncType
	)

	if pos = astcodec.FileFindDecl[*ast.FuncDecl](tree, astcodec.FindFunctionsByName(name)); pos == nil {
		return nil, fmt.Errorf("unable to locate function declaration for insert: %s", name)
	}

	log.Printf("FunctionFromFile: %s\n", nodeInfo(cctx, pos))
	// rewrite scanner declaration function.
	if declPattern, ok = pos.Type.Params.List[1].Type.(*ast.FuncType); !ok {
		return nil, errorsx.String("genieql.Function second parameter must be a function type")
	}

	if scanner = functions.DetectScanner(cctx, declPattern); scanner == nil {
		return nil, errorsx.Errorf("genieql.Function %s - missing scanner", nodeInfo(cctx, pos))
	}

	return NewFunction(
		cctx,
		name,
		declPattern,
		pos.Doc,
	), nil
}

type function struct {
	ctx       generators.Context
	name      string
	signature *ast.FuncType
	comment   *ast.CommentGroup
	query     string
}

func (t *function) Query(q string) Function {
	t.query = q
	return t
}

func (t *function) Generate(dst io.Writer) (err error) {
	var (
		n            *ast.FuncDecl
		cf           *ast.Field
		qf           *ast.Field
		cmaps        []genieql.ColumnMap
		qinputs      []ast.Expr
		encodings    []ast.Stmt
		locals       []ast.Spec
		transforms   []ast.Stmt
		encodedquery string
	)

	t.ctx.Println("generation of", t.name, "initiated")
	defer t.ctx.Println("generation of", t.name, "completed")

	if cf = functions.DetectContext(t.signature); cf != nil {
		// pop the context off the params.
		t.signature.Params.List = t.signature.Params.List[1:]
	}

	if len(t.signature.Params.List) < 1 {
		return errorsx.New("functions must start with a queryer param")
	}

	// pop the queryer off the params.
	qf = t.signature.Params.List[0]
	t.signature.Params.List = generators.NormalizeFieldNames(t.signature.Params.List[1:]...)

	scanner := functions.DetectScanner(t.ctx, t.signature)

	if cmaps, err = generators.ColumnMapFromFields(t.ctx, t.signature.Params.List...); err != nil {
		return errorsx.Wrap(err, "unable to generate mapping")
	}

	encodedquery, cmaps = functions.ColumnUsageFilter(t.ctx, t.query, cmaps...)
	if locals, encodings, qinputs, err = generators.QueryInputsFromColumnMap(t.ctx, scanner, nil, cmaps...); err != nil {
		return errorsx.Wrap(err, "unable to transform query inputs")
	}

	if len(locals) > 0 {
		transforms = []ast.Stmt{
			&ast.DeclStmt{
				Decl: astutil.VarList(locals...),
			},
		}
	}

	transforms = append(transforms, encodings...)

	qfn := functions.Query{
		Context: t.ctx,
		Query: astutil.StringLiteral(
			encodedquery,
		),
		Scanner:      scanner,
		Queryer:      qf.Type,
		ContextField: cf,
		Transforms:   transforms,
		QueryInputs:  qinputs,
	}

	if n, err = qfn.Compile(functions.New(t.name, t.signature)); err != nil {
		return err
	}

	if err = generators.GenerateComment(generators.DefaultFunctionComment(t.name), t.comment).Generate(dst); err != nil {
		return err
	}

	if err = printer.Fprint(dst, t.ctx.FileSet, n); err != nil {
		return err
	}

	return nil
}
