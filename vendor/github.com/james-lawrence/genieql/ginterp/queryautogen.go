package ginterp

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"strings"

	"github.com/serenize/snaker"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/astcodec"
	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/generators"
	"github.com/james-lawrence/genieql/generators/functions"
	"github.com/james-lawrence/genieql/internal/errorsx"
)

// QueryAutogen configuration interface for generating basic queries automatically.
type QueryAutogen interface {
	genieql.Generator              // must satisfy the generator interface
	From(string) QueryAutogen      // what table to insert into
	Ignore(...string) QueryAutogen // ignore the specified columns.
}

func QueryAutogenFromFile(cctx generators.Context, name string, tree *ast.File) (QueryAutogen, error) {
	var (
		pos     *ast.FuncDecl
		cf      *ast.Field    // context field
		qf      *ast.Field    // query field
		typ     *ast.Field    // type we're scanning into the table.
		scanner *ast.FuncDecl // scanner to use for the results.
	)

	if pos = astcodec.FileFindDecl[*ast.FuncDecl](tree, astcodec.FindFunctionsByName(name)); pos == nil {
		return nil, fmt.Errorf("genieql.QueryAutogen unable to locate function declaration: %s", name)
	}

	// pop off the genieql.QueryAutogen
	pos.Type.Params.List = pos.Type.Params.List[1:]

	if cf = functions.DetectContext(pos.Type); cf != nil {
		// pop the context off the params.
		pos.Type.Params.List = pos.Type.Params.List[1:]
	}

	qf, typ = pos.Type.Params.List[0], pos.Type.Params.List[1]

	if scanner = functions.DetectScanner(cctx, pos.Type); scanner == nil {
		return nil, errorsx.Errorf("genieql.QueryAutogen %s - missing scanner", nodeInfo(cctx, pos))
	}

	return NewQueryAutogen(
		cctx,
		pos.Name.String(),
		pos.Doc,
		cf,
		qf,
		typ,
		scanner,
	), nil
}

// NewQueryAutogen instantiate a query autogen. which generates basic queries
// from the provided type and table.
func NewQueryAutogen(
	ctx generators.Context,
	name string,
	comment *ast.CommentGroup,
	cf *ast.Field,
	qf *ast.Field,
	tf *ast.Field,
	scanner *ast.FuncDecl,
) QueryAutogen {
	return &queryAutogen{
		ctx:     ctx,
		name:    name,
		comment: comment,
		qf:      qf,
		cf:      cf,
		tf:      tf,
		scanner: scanner,
	}
}

type queryAutogen struct {
	ctx     generators.Context
	name    string
	table   string
	ignore  []string
	tf      *ast.Field    // type field.
	cf      *ast.Field    // context field, can be nil.
	qf      *ast.Field    // db Query field.
	scanner *ast.FuncDecl // scanner being used for results.
	comment *ast.CommentGroup
}

// Into specify the table the data will be inserted into.
func (t *queryAutogen) From(s string) QueryAutogen {
	t.table = s
	return t
}

// Ingore specify the table columns to ignore.
func (t *queryAutogen) Ignore(ignore ...string) QueryAutogen {
	t.ignore = ignore
	return t
}

func (t *queryAutogen) Generate(dst io.Writer) (err error) {
	var (
		mapping genieql.MappingConfig
		details genieql.TableDetails
	)

	t.ctx.Println("generation of", t.name, "initiated")
	defer t.ctx.Println("generation of", t.name, "completed")
	t.ctx.Debugln("type", types.ExprString(t.tf.Type))
	t.ctx.Debugln("table", t.table)
	t.ctx.Debugln("package", t.ctx.CurrentPackage.Name)
	t.ctx.Debugln("import path", t.ctx.CurrentPackage.ImportPath)

	if strings.TrimSpace(t.table) == "" {
		return errorsx.Errorf(
			"%s:%s - table is required. use From method to specify a table",
			t.ctx.CurrentPackage.Name,
			types.ExprString(t.tf.Type),
		)
	}

	err = t.ctx.Configuration.ReadMap(
		&mapping,
		genieql.MCOPackage(t.ctx.CurrentPackage),
		genieql.MCOType(types.ExprString(t.tf.Type)),
	)

	if err != nil {
		return err
	}

	if details, err = genieql.LookupTableDetails(t.ctx.Driver, t.ctx.Dialect, t.table); err != nil {
		return err
	}

	mg := make([]genieql.Generator, 0, 10)
	ignore := genieql.ColumnInfoFilterIgnore(t.ignore...)
	names := genieql.ColumnInfoSet(details.Columns).ColumnNames()
	// naturalKey := genieql.ColumnInfoSet(details.Columns).PrimaryKey()
	defaults := functions.Query{
		Context:      t.ctx,
		Scanner:      t.scanner,
		Queryer:      t.qf.Type,
		ContextField: t.cf,
	}

	for i, c := range genieql.ColumnInfoSet(details.Columns).Filter(ignore) {
		column := c
		idx := i
		name := t.name + snaker.SnakeToCamel(column.Name)
		g := genieql.NewFuncGenerator(func(dst io.Writer) (err error) {
			var (
				n ast.Node
			)

			query := details.Dialect.Select(details.Table, names, genieql.ColumnInfoSet(details.Columns[idx:idx+1]).ColumnNames())
			qfn := defaults
			qfn.Query = astutil.StringLiteral(query)

			sig := &ast.FuncType{
				Params: &ast.FieldList{
					List: []*ast.Field{astutil.Field(ast.NewIdent(column.Definition.ColumnType), ast.NewIdent("c"))},
				},
			}
			if err = generators.GenerateComment(generators.DefaultFunctionComment(name)).Generate(dst); err != nil {
				return err
			}
			if n, err = qfn.Compile(functions.New(name, sig)); err != nil {
				return err
			}

			return printer.Fprint(dst, token.NewFileSet(), n)
		})

		mg = append(mg, g)
	}

	// if len(naturalKey) > 0 {
	// 	query := details.Dialect.Select(details.Table, names, naturalKey.ColumnNames())
	// 	options = []generators.QueryFunctionOption{
	// 		queryerOption,
	// 		generators.QFOSharedParameters(fieldFromColumnInfo(naturalKey...)...),
	// 		generators.QFOBuiltinQueryFromString(query),
	// 		generators.QFOName(fmt.Sprintf("%sFindByKey", t.Type)),
	// 		generators.QFOScanner(t.UniqScanner),
	// 	}
	// 	mg = append(mg, generators.NewQueryFunction(options...))
	// 	mg = append(mg, t.updateFunc(queryerOption, naturalKey, names))
	// 	query = details.Dialect.Delete(details.Table, names, naturalKey.ColumnNames())
	// 	options = []generators.QueryFunctionOption{
	// 		queryerOption,
	// 		generators.QFOSharedParameters(fieldFromColumnInfo(naturalKey...)...),
	// 		generators.QFOBuiltinQueryFromString(query),
	// 		generators.QFOName(fmt.Sprintf("%sDeleteByID", t.Type)),
	// 		generators.QFOScanner(t.UniqScanner),
	// 	}
	// 	mg = append(mg, generators.NewQueryFunction(options...))
	// }

	return genieql.MultiGenerate(mg...).Generate(dst)
}
