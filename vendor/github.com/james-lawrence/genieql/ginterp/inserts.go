package ginterp

import (
	"fmt"
	"go/ast"
	"go/types"
	"io"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/astcodec"
	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/generators"
	"github.com/james-lawrence/genieql/generators/functions"
	"github.com/james-lawrence/genieql/internal/errorsx"
)

// Insert configuration interface for generating Insert.
type Insert interface {
	genieql.Generator         // must satisfy the generator interface
	Into(string) Insert       // what table to insert into
	Ignore(...string) Insert  // do not attempt to insert the specified column.
	Default(...string) Insert // use the database default for the specified columns.
	Conflict(string) Insert   // specify how conflicts should be handled.
}

func InsertFromFile(cctx generators.Context, name string, tree *ast.File) (Insert, error) {
	var (
		ok          bool
		declPattern *ast.FuncType
		pos         *ast.FuncDecl
		scanner     *ast.FuncDecl // scanner to use for the results.
		cf          *ast.Field
		qf          *ast.Field
		tf          *ast.Field
		params      []*ast.Field
	)

	if pos = astcodec.FileFindDecl[*ast.FuncDecl](tree, astcodec.FindFunctionsByName(name)); pos == nil {
		return nil, fmt.Errorf("unable to locate function declaration for insert: %s", name)
	}

	// rewrite scanner declaration function.
	if declPattern, ok = pos.Type.Params.List[1].Type.(*ast.FuncType); !ok {
		return nil, errorsx.String("genieql.Insert second parameter must be a function type")
	}

	if scanner = functions.DetectScanner(cctx, declPattern); scanner == nil {
		return nil, errorsx.Errorf("genieql.Insert %s - missing scanner", nodeInfo(cctx, pos))
	}

	if cf = functions.DetectContext(declPattern); cf != nil {
		declPattern.Params.List = declPattern.Params.List[1:]
	}

	if qf = functions.DetectQueryer(declPattern); qf != nil {
		declPattern.Params.List = declPattern.Params.List[1:]
	}

	switch plen := len(declPattern.Params.List); plen {
	case 0:
		return nil, errorsx.Errorf("genieql.Insert %s - missing type to insert; should be the last parameter of function declaration argument", nodeInfo(cctx, pos))
	case 1:
		tf = declPattern.Params.List[0]
		params = declPattern.Params.List
	default:
		tf = declPattern.Params.List[plen-1]
		params = declPattern.Params.List
	}

	return NewInsert(
		cctx,
		pos.Name.String(),
		pos.Doc,
		scanner,
		cf,
		qf,
		tf,
		params...,
	), nil
}

// NewInsert instantiate a new insert generator. it uses the name of function
// that calls Define as the name of the generated function.
func NewInsert(
	ctx generators.Context,
	name string,
	comment *ast.CommentGroup,
	scanner *ast.FuncDecl,
	cf *ast.Field,
	qf *ast.Field,
	tf *ast.Field,
	params ...*ast.Field,
) Insert {
	return &insert{
		ctx:     ctx,
		name:    name,
		comment: comment,
		qf:      qf,
		cf:      cf,
		tf:      tf,
		params:  params,
		scanner: scanner,
	}
}

type insert struct {
	ctx      generators.Context
	name     string
	table    string
	conflict string
	defaults []string
	ignore   []string
	params   []*ast.Field
	tf       *ast.Field    // type field.
	cf       *ast.Field    // context field, can be nil.
	qf       *ast.Field    // db Query field.
	scanner  *ast.FuncDecl // scanner being used for results.
	comment  *ast.CommentGroup
}

// Into specify the table the data will be inserted into.
func (t *insert) Into(s string) Insert {
	t.table = s
	return t
}

// Default specify the table columns to be given their default values.
func (t *insert) Default(defaults ...string) Insert {
	t.defaults = defaults
	return t
}

// Ignore specify the table columns to ignore during insert.
// - ignored columns should be defaulted in the static columns.
// - ignored columns should not be read from the structures during explode.
// - ignored columns should not be returned by the query.
func (t *insert) Ignore(ignore ...string) Insert {
	t.ignore = ignore
	return t
}

func (t *insert) Conflict(s string) Insert {
	t.conflict = s
	return t
}

func (t *insert) Generate(dst io.Writer) (err error) {
	var (
		insertcmaps []genieql.ColumnMap
		paramscmaps []genieql.ColumnMap
		qinputs     []ast.Expr
		encodings   []ast.Stmt
		locals      []ast.Spec
		transforms  []ast.Stmt
	)

	dialect := t.ctx.Dialect

	t.ctx.Println("generation of", t.name, "initiated")
	defer t.ctx.Println("generation of", t.name, "completed")
	t.ctx.Debugln("insert type", t.ctx.CurrentPackage.Name, t.ctx.CurrentPackage.ImportPath, types.ExprString(t.tf.Type))
	t.ctx.Debugln("insert table", t.table)

	if paramscmaps, err = generators.ColumnMapFromFields(t.ctx, t.params...); err != nil {
		return errorsx.Wrap(err, "unable to generate mapping")
	}

	if insertcmaps, err = generators.ColumnMapFromFields(t.ctx, t.tf); err != nil {
		return errorsx.Wrap(err, "unable to generate mapping")
	}

	ignored := genieql.ColumnInfoFilterIgnore(t.ignore...)
	defaulted := genieql.ColumnInfoFilterIgnore(t.defaults...)

	cset0 := genieql.ColumnMapSet(paramscmaps)
	ignoredcset0 := cset0.Filter(func(cm genieql.ColumnMap) bool { return ignored(cm.ColumnInfo) })
	projectioncset0 := ignoredcset0.Filter(func(cm genieql.ColumnMap) bool { return defaulted(cm.ColumnInfo) })

	if locals, encodings, qinputs, err = generators.QueryInputsFromColumnMap(t.ctx, t.scanner, nil, projectioncset0...); err != nil {
		return errorsx.Wrap(err, "unable to transform query inputs")
	}

	transforms = []ast.Stmt{
		&ast.DeclStmt{
			Decl: astutil.VarList(locals...),
		},
	}
	transforms = append(transforms, encodings...)

	cset := genieql.ColumnMapSet(insertcmaps)
	ignoredcset := cset.Filter(func(cm genieql.ColumnMap) bool { return ignored(cm.ColumnInfo) })
	projectioncset := ignoredcset.Filter(func(cm genieql.ColumnMap) bool { return defaulted(cm.ColumnInfo) })

	g1 := generators.NewColumnConstants(
		fmt.Sprintf("%sStaticColumns", t.name),
		genieql.ColumnValueTransformer{
			Defaults:           append(t.defaults, t.ignore...),
			DialectTransformer: dialect.ColumnValueTransformer(),
		},
		cset.ColumnInfo(),
	)

	g2 := generators.NewExploderFunction(
		t.ctx,
		astutil.Field(ast.NewIdent(types.ExprString(t.tf.Type)), t.tf.Names...),
		projectioncset,
		generators.QFOName(fmt.Sprintf("%sExplode", t.name)),
	)

	qfn := functions.Query{
		Context:      t.ctx,
		Scanner:      t.scanner,
		Queryer:      t.qf.Type,
		Transforms:   transforms,
		QueryInputs:  qinputs,
		ContextField: t.cf,
		Query: astutil.StringLiteral(
			functions.QueryLiteralColumnMapReplacer(
				t.ctx,
				dialect.Insert(
					1,
					len(paramscmaps)-len(insertcmaps),
					t.table,
					t.conflict,
					cset.ColumnNames(),
					ignoredcset.ColumnNames(),
					append(t.defaults, t.ignore...),
				),
				projectioncset0...,
			),
		),
	}

	sig := &ast.FuncType{
		Params: &ast.FieldList{
			List: astutil.FlattenFields(t.params...),
		},
	}

	return genieql.MultiGenerate(
		g1,
		g2,
		genieql.NewFuncGenerator(func(dst io.Writer) (err error) {
			if err = generators.GenerateComment(generators.DefaultFunctionComment(t.name), t.comment).Generate(dst); err != nil {
				return err
			}

			if err = functions.CompileInto(dst, functions.New(t.name, sig), qfn); err != nil {
				return err
			}

			return nil
		}),
	).Generate(dst)
}
