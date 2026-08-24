package ginterp

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"strings"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/astcodec"
	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/generators"
	"github.com/james-lawrence/genieql/generators/functions"
	"github.com/james-lawrence/genieql/generators/typespec"
	"github.com/james-lawrence/genieql/internal/errorsx"
	"github.com/james-lawrence/genieql/internal/stringsx"
)

// InsertBatch configuration interface for generating batch inserts.
type InsertBatch interface {
	genieql.Generator              // must satisfy the generator interface
	Into(string) InsertBatch       // what table to insert into
	Default(...string) InsertBatch // use the database default for the specified columns.
	Conflict(string) InsertBatch   // specify how conflicts should be handled.
	Batch(n int) InsertBatch       // specify a batch insert
}

// NewInsert instantiate a new insert generator. it uses the name of function
// that calls Define as the name of the generated function.
func NewBatchInsert(
	ctx generators.Context,
	name string,
	comment *ast.CommentGroup,
	cf *ast.Field,
	qf *ast.Field,
	tf *ast.Field,
	scanner *ast.FuncDecl,
) InsertBatch {
	return &batch{
		ctx:     ctx,
		name:    name,
		comment: comment,
		qf:      qf,
		cf:      cf,
		tf:      tf,
		scanner: scanner,
		n:       1,
	}
}

func InsertBatchFromFile(cctx generators.Context, name string, tree *ast.File) (InsertBatch, error) {
	var (
		ok          bool
		pos         *ast.FuncDecl
		scanner     *ast.FuncDecl // scanner to use for the results.
		declPattern *ast.FuncType
	)

	if pos = astcodec.FileFindDecl[*ast.FuncDecl](tree, astcodec.FindFunctionsByName(name)); pos == nil {
		return nil, fmt.Errorf("unable to locate function declaration for insert: %s", name)
	}

	// rewrite scanner declaration function.
	if declPattern, ok = pos.Type.Params.List[1].Type.(*ast.FuncType); !ok {
		return nil, errorsx.String("InsertBatch second parameter must be a function type")
	}

	if scanner = functions.DetectScanner(cctx, declPattern); scanner == nil {
		return nil, errorsx.Errorf("InsertBatch %s - missing scanner", nodeInfo(cctx, pos))
	}

	return NewBatchInsert(
		cctx,
		pos.Name.String(),
		pos.Doc,
		functions.DetectContext(declPattern),
		functions.DetectQueryer(declPattern),
		declPattern.Params.List[len(declPattern.Params.List)-1],
		scanner,
	), nil
}

type batch struct {
	ctx      generators.Context
	n        int // number of records to support inserting
	name     string
	table    string
	conflict string
	defaults []string
	tf       *ast.Field    // type field.
	cf       *ast.Field    // context field, can be nil.
	qf       *ast.Field    // db Query field.
	scanner  *ast.FuncDecl // scanner being used for results.
	comment  *ast.CommentGroup
}

// Into specify the table the data will be inserted into.
func (t *batch) Into(s string) InsertBatch {
	t.table = s
	return t
}

// Default specify the table columns to be given their default values.
func (t *batch) Default(defaults ...string) InsertBatch {
	t.defaults = defaults
	return t
}

// Conflict specify how to handle conflict during an insert.
func (t *batch) Conflict(s string) InsertBatch {
	t.conflict = s
	return t
}

// Batch specify the maximum number of records to insert.
func (t *batch) Batch(size int) InsertBatch {
	t.n = size
	return t
}

func (t *batch) Generate(dst io.Writer) (err error) {
	var (
		cmaps       []genieql.ColumnMap
		queryfields []*ast.Field
		encodings   []ast.Stmt
		explodedecl *ast.FuncDecl
	)
	t.ctx.Println("generation of", t.name, "initiated")
	defer t.ctx.Println("generation of", t.name, "completed")
	t.ctx.Debugln("batch.insert type", t.ctx.CurrentPackage.Name, t.ctx.CurrentPackage.ImportPath, types.ExprString(t.tf.Type))
	t.ctx.Debugln("batch.insert table", t.table)
	t.ctx.Debugln("batch.insert type", t.tf.Names[0])
	t.ctx.Debugln("batch.insert scanner", t.scanner)

	if cmaps, err = generators.ColumnMapFromFields(t.ctx, t.tf); err != nil {
		return errorsx.Wrap(err, "unable to generate mapping")
	}

	defaulted := genieql.ColumnInfoFilterIgnore(t.defaults...)

	cset := genieql.ColumnMapSet(cmaps)
	defaultedcset := cset.Filter(func(cm genieql.ColumnMap) bool { return defaulted(cm.ColumnInfo) })

	queryfields = generators.QueryFieldsFromColumnMap(t.ctx, defaultedcset.Map(func(idx int, cm genieql.ColumnMap) genieql.ColumnMap {
		local := cm.Local(idx)
		dup := cm
		dup.Field = astutil.Field(astutil.MustParseExpr(t.ctx.FileSet, cm.Definition.ColumnType), local)
		return dup
	})...)

	explodeerrHandler := func(errlocal string) ast.Node {
		explodereturn := make([]ast.Expr, 0, len(queryfields)+1)
		explodereturn = append(explodereturn, astutil.MapFieldsToNameExpr(queryfields...)...)
		explodereturn = append(explodereturn, ast.NewIdent(errlocal))
		return astutil.Return(explodereturn...)
	}

	if _, encodings, _, err = generators.QueryInputsFromColumnMap(t.ctx, t.scanner, explodeerrHandler, defaultedcset...); err != nil {
		return errorsx.Wrap(err, "unable to transform query inputs")
	}

	errhandling := generators.ScannerErrorHandlingExpr(t.scanner)

	initializesig := &ast.FuncType{
		Params: &ast.FieldList{
			List: []*ast.Field{
				t.cf,
				t.qf,
				astutil.Field(&ast.Ellipsis{
					Elt: t.tf.Type,
				}, t.tf.Names...),
			},
		},
		Results: t.scanner.Type.Results,
	}

	typename := stringsx.ToPrivate(t.name)
	initialize := functions.NewFn(
		astutil.Return(
			&ast.UnaryExpr{
				Op: token.AND,
				X: &ast.CompositeLit{
					Type: ast.NewIdent(typename),
					Elts: []ast.Expr{
						&ast.KeyValueExpr{
							Key:   t.cf.Names[0],
							Value: t.cf.Names[0],
						},
						&ast.KeyValueExpr{
							Key:   t.qf.Names[0],
							Value: t.qf.Names[0],
						},
						&ast.KeyValueExpr{
							Key:   ast.NewIdent("remaining"),
							Value: t.tf.Names[0],
						},
					},
				},
			},
		),
	)

	typedecl := typespec.NewType(typename, &ast.StructType{
		Struct: token.Pos(0),
		Fields: &ast.FieldList{
			List: []*ast.Field{
				t.cf,
				t.qf,
				astutil.Field(
					&ast.ArrayType{Elt: t.tf.Type}, ast.NewIdent("remaining")),
				astutil.Field(t.scanner.Type.Results.List[0].Type, ast.NewIdent("scanner")),
			},
		},
	})

	fnrecv := astutil.FieldList(astutil.Field(&ast.StarExpr{X: astutil.Expr(typename)}, ast.NewIdent("t")))

	scansig := &ast.FuncType{
		Params: &ast.FieldList{
			List: []*ast.Field{
				astutil.Field(&ast.StarExpr{X: t.tf.Type}, t.tf.Names...),
			},
		},
		Results: &ast.FieldList{
			List: []*ast.Field{
				astutil.Field(ast.NewIdent("error")),
			},
		},
	}
	scanfn := functions.NewFn(
		astutil.Return(
			astutil.CallExpr(
				&ast.SelectorExpr{
					X: astutil.SelExpr(
						"t", "scanner",
					),
					Sel: ast.NewIdent("Scan"),
				},
				t.tf.Names[0],
			),
		),
	)

	errsig := &ast.FuncType{
		Params: &ast.FieldList{},
		Results: &ast.FieldList{
			List: []*ast.Field{
				astutil.Field(ast.NewIdent("error")),
			},
		},
	}

	errfn := functions.NewFn(
		astutil.If(
			nil,
			astutil.BinaryExpr(astutil.SelExpr("t", "scanner"), token.EQL, ast.NewIdent("nil")),
			astutil.Block(
				astutil.Return(ast.NewIdent("nil")),
			),
			nil,
		),
		astutil.Return(
			astutil.CallExpr(
				astutil.SelExpr(
					types.ExprString(
						astutil.SelExpr(
							"t", "scanner",
						),
					),
					"Err",
				),
			),
		),
	)

	closesig := &ast.FuncType{
		Params: &ast.FieldList{},
		Results: &ast.FieldList{
			List: []*ast.Field{
				astutil.Field(ast.NewIdent("error")),
			},
		},
	}

	closefn := functions.NewFn(
		astutil.If(
			nil,
			astutil.BinaryExpr(astutil.SelExpr("t", "scanner"), token.EQL, ast.NewIdent("nil")),
			astutil.Block(
				astutil.Return(ast.NewIdent("nil")),
			),
			nil,
		),
		astutil.Return(
			astutil.CallExpr(
				astutil.SelExpr(
					types.ExprString(
						astutil.SelExpr(
							"t", "scanner",
						),
					),
					"Close",
				),
			),
		),
	)

	nextsig := &ast.FuncType{
		Params: &ast.FieldList{},
		Results: &ast.FieldList{
			List: []*ast.Field{
				astutil.Field(ast.NewIdent("bool")),
			},
		},
	}

	nextfn := functions.NewFn(
		astutil.DeclStmt(
			astutil.VarList(
				astutil.ValueSpec(ast.NewIdent("bool"), ast.NewIdent("advanced")),
			),
		),
		astutil.If(
			nil, astutil.BinaryExpr(
				astutil.BinaryExpr(astutil.SelExpr("t", "scanner"), token.NEQ, ast.NewIdent("nil")),
				token.LAND,
				astutil.CallExpr(
					&ast.SelectorExpr{
						X:   astutil.SelExpr("t", "scanner"),
						Sel: ast.NewIdent("Next"),
					},
				),
			),
			astutil.Block(
				astutil.Return(ast.NewIdent("true")),
			),
			nil,
		),
		astutil.If(
			nil, astutil.BinaryExpr(
				astutil.BinaryExpr(astutil.CallExpr(ast.NewIdent("len"), astutil.SelExpr("t", "remaining")), token.GTR, astutil.IntegerLiteral(0)),
				token.LAND,
				astutil.BinaryExpr(
					astutil.CallExpr(
						astutil.SelExpr("t", "Close"),
					),
					token.EQL,
					ast.NewIdent("nil"),
				),
			),
			astutil.Block(
				astutil.Assign(
					astutil.ExprList(
						astutil.SelExpr("t", "scanner"),
						astutil.SelExpr("t", "remaining"),
						ast.NewIdent("advanced"),
					),
					token.ASSIGN,
					astutil.ExprList(
						astutil.CallExprEllipsis(
							astutil.SelExpr("t", "advance"),
							astutil.SelExpr("t", "remaining"),
						),
					),
				),
				astutil.Return(
					astutil.BinaryExpr(ast.NewIdent("advanced"), token.LAND, astutil.CallExpr(&ast.SelectorExpr{
						X:   astutil.SelExpr("t", "scanner"),
						Sel: ast.NewIdent("Next"),
					})),
				),
			),
			nil,
		),
		astutil.Return(
			ast.NewIdent("false"),
		),
	)

	advancesig := &ast.FuncType{
		Params: &ast.FieldList{
			List: []*ast.Field{
				astutil.Field(&ast.Ellipsis{Elt: t.tf.Type}, t.tf.Names[0]),
			},
		},
		Results: &ast.FieldList{
			List: []*ast.Field{
				t.scanner.Type.Results.List[0],
				astutil.Field(&ast.ArrayType{Elt: t.tf.Type}),
				astutil.Field(ast.NewIdent("bool")),
			},
		},
	}

	explodereturn := make([]ast.Expr, 0, len(queryfields)+1)
	explodereturn = append(explodereturn, astutil.MapFieldsToNameExpr(queryfields...)...)
	explodereturn = append(explodereturn, ast.NewIdent("nil"))
	explodestmts := make([]ast.Stmt, 0, len(encodings)+1)
	explodestmts = append(explodestmts, encodings...)
	explodestmts = append(explodestmts, astutil.Return(explodereturn...))

	explodefn := functions.NewFn(
		explodestmts...,
	)

	exploderesults := make([]*ast.Field, len(queryfields), len(queryfields)+1)
	copy(exploderesults, queryfields)
	exploderesults = append(exploderesults, astutil.Field(ast.NewIdent("error"), ast.NewIdent("err")))

	explodesig := &ast.FuncType{
		Params: &ast.FieldList{
			List: []*ast.Field{
				t.tf,
			},
		},
		Results: &ast.FieldList{
			List: exploderesults,
		},
	}

	if explodedecl, err = explodefn.Compile(functions.New("", explodesig)); err != nil {
		return errorsx.Wrap(err, "failed to generate encoding function")
	}

	valueTupleExprs := make([]ast.Expr, t.n)

	qi := functions.QueryLiteralColumnMapReplacer(t.ctx, t.ctx.Dialect.Insert(t.n, 0, t.table, t.conflict, cset.ColumnNames(), cset.ColumnNames(), t.defaults), cmaps...)
	queryPrefix, remaining, _ := strings.Cut(qi, "VALUES")
	queryPrefix += "VALUES "
	querySuffix := ""
	tuples, suffix, ok := strings.Cut(remaining, " ON CONFLICT ")
	if ok {
		querySuffix = " ON CONFLICT " + suffix
	}
	tuples, suffix, ok = strings.Cut(tuples, " RETURNING ")
	if ok {
		querySuffix = " RETURNING " + suffix
	}

	tuples = strings.ReplaceAll(tuples, "),(", ")),((")
	tuplesarr := strings.Split(tuples, "),(")
	for i := range t.n {
		valueTupleExprs[i] = astutil.StringLiteral(strings.TrimSpace(tuplesarr[i]))
	}

	colIdents := astutil.MapFieldsToNameExpr(queryfields...)
	transformLHS := append(astutil.MapFieldsToNameExpr(queryfields...), ast.NewIdent("err"))
	appendCallArgs := append([]ast.Expr{ast.NewIdent("args")}, colIdents...)

	loopbody := astutil.Block(
		astutil.Assign(
			transformLHS,
			token.DEFINE,
			astutil.ExprList(
				astutil.CallExpr(
					ast.NewIdent("transform"),
					&ast.IndexListExpr{
						X:       t.tf.Names[0],
						Indices: astutil.ExprList(ast.NewIdent("i")),
					},
				),
			),
		),
		astutil.If(
			nil,
			astutil.BinaryExpr(ast.NewIdent("err"), token.NEQ, ast.NewIdent("nil")),
			astutil.Block(
				astutil.Return(
					errhandling("err"),
					astutil.CallExpr(&ast.ArrayType{Elt: t.tf.Type}, ast.NewIdent("nil")),
					ast.NewIdent("false"),
				),
			),
			nil,
		),
		astutil.Assign(
			astutil.ExprList(ast.NewIdent("args")),
			token.ASSIGN,
			astutil.ExprList(astutil.CallExpr(ast.NewIdent("append"), appendCallArgs...)),
		),
	)

	advancefn := functions.NewFn(
		astutil.Assign(
			astutil.ExprList(ast.NewIdent("transform")),
			token.DEFINE,
			astutil.ExprList(astutil.FuncLiteral(explodedecl)),
		),
		astutil.If(
			nil,
			astutil.BinaryExpr(
				astutil.CallExpr(ast.NewIdent("len"), t.tf.Names[0]),
				token.EQL,
				astutil.IntegerLiteral(0),
			),
			astutil.Block(
				astutil.Return(
					ast.NewIdent("nil"),
					astutil.CallExpr(&ast.ArrayType{Elt: t.tf.Type}, ast.NewIdent("nil")),
					ast.NewIdent("false"),
				),
			),
			nil,
		),
		astutil.Assign(
			astutil.ExprList(ast.NewIdent("n")),
			token.DEFINE,
			astutil.ExprList(
				astutil.CallExpr(
					ast.NewIdent("min"),
					astutil.CallExpr(ast.NewIdent("len"), t.tf.Names[0]),
					astutil.IntegerLiteral(t.n),
				),
			),
		),
		astutil.DeclStmt(genieql.QueryLiteral("queryPrefix", queryPrefix)),
		astutil.DeclStmt(genieql.QueryLiteral("querySuffix", querySuffix)),
		astutil.Assign(
			astutil.ExprList(ast.NewIdent("valueTuples")),
			token.DEFINE,
			astutil.ExprList(
				&ast.CompositeLit{
					Type: &ast.ArrayType{
						Len: astutil.IntegerLiteral(t.n),
						Elt: ast.NewIdent("string"),
					},
					Elts: valueTupleExprs,
				},
			),
		),
		astutil.Assign(
			astutil.ExprList(ast.NewIdent("query")),
			token.DEFINE,
			astutil.ExprList(
				astutil.BinaryExpr(
					astutil.BinaryExpr(
						ast.NewIdent("queryPrefix"),
						token.ADD,
						astutil.CallExpr(
							&ast.SelectorExpr{X: ast.NewIdent("strings"), Sel: ast.NewIdent("Join")},
							&ast.SliceExpr{X: ast.NewIdent("valueTuples"), High: ast.NewIdent("n")},
							astutil.StringLiteral(","),
						),
					),
					token.ADD,
					ast.NewIdent("querySuffix"),
				),
			),
		),
		astutil.Assign(
			astutil.ExprList(ast.NewIdent("args")),
			token.DEFINE,
			astutil.ExprList(
				astutil.CallExpr(
					ast.NewIdent("make"),
					&ast.ArrayType{Elt: ast.NewIdent("any")},
					astutil.IntegerLiteral(0),
					astutil.BinaryExpr(
						ast.NewIdent("n"),
						token.MUL,
						astutil.IntegerLiteral(len(queryfields)),
					),
				),
			),
		),
		astutil.Range(
			ast.NewIdent("i"),
			nil,
			token.DEFINE,
			ast.NewIdent("n"),
			loopbody,
		),
		astutil.Return(
			astutil.CallExpr(
				t.scanner.Name,
				astutil.CallExprEllipsis(
					&ast.SelectorExpr{
						X:   astutil.SelExpr("t", "q"),
						Sel: ast.NewIdent("QueryContext"),
					},
					ast.NewIdent("t.ctx"),
					ast.NewIdent("query"),
					ast.NewIdent("args"),
				),
			),
			&ast.SliceExpr{
				X:   t.tf.Names[0],
				Low: ast.NewIdent("n"),
			},
			ast.NewIdent("true"),
		),
	)

	return genieql.NewFuncGenerator(func(dst io.Writer) (err error) {
		if err = generators.GenerateComment(generators.DefaultFunctionComment(t.name), t.comment).Generate(dst); err != nil {
			return err
		}

		if err = functions.CompileInto(dst, functions.New("New"+stringsx.ToPublic(t.name), initializesig), initialize); err != nil {
			return err
		}

		if err = generators.GapLines(dst, 2); err != nil {
			return err
		}

		if err = typespec.CompileInto(dst, typedecl); err != nil {
			return err
		}

		if err = generators.GapLines(dst, 2); err != nil {
			return err
		}

		if err = functions.CompileInto(dst, functions.New("Scan", scansig, functions.OptionRecv(fnrecv)), scanfn); err != nil {
			return err
		}

		if err = generators.GapLines(dst, 2); err != nil {
			return err
		}

		if err = functions.CompileInto(dst, functions.New("Err", errsig, functions.OptionRecv(fnrecv)), errfn); err != nil {
			return err
		}

		if err = generators.GapLines(dst, 2); err != nil {
			return err
		}

		if err = functions.CompileInto(dst, functions.New("Close", closesig, functions.OptionRecv(fnrecv)), closefn); err != nil {
			return err
		}

		if err = generators.GapLines(dst, 2); err != nil {
			return err
		}

		if err = functions.CompileInto(dst, functions.New("Next", nextsig, functions.OptionRecv(fnrecv)), nextfn); err != nil {
			return err
		}

		if err = generators.GapLines(dst, 2); err != nil {
			return err
		}

		if err = functions.CompileInto(dst, functions.New("advance", advancesig, functions.OptionRecv(fnrecv)), advancefn); err != nil {
			return err
		}

		return nil
	}).Generate(dst)
}
