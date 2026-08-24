package generators

import (
	"go/ast"
	"go/build"
	"go/types"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/astcodec"
	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/buildx"
	"github.com/james-lawrence/genieql/internal/stringsx"
	"github.com/james-lawrence/genieql/internal/transformx"
)

// mappedParam converts a *ast.Field that represents a struct into an array
// of ColumnInfo.
func mappedParam(ctx Context, param *ast.Field) (m genieql.MappingConfig, infos []genieql.ColumnInfo, err error) {
	var (
		pkg *build.Package = ctx.CurrentPackage
	)

	if ipath, err := importPath(ctx, astutil.UnwrapExpr(param.Type)); err != nil {
		return m, infos, err
	} else if ipath != ctx.CurrentPackage.ImportPath {
		// when scanning for types we need to reset the build tags to
		// ensure we see the generated code for the other package.
		sbtx := buildx.Clone(ctx.Build, buildx.Tags())

		if pkg, err = astcodec.LocatePackage(ipath, ".", sbtx, genieql.StrictPackageImport(ipath)); err != nil {
			return m, infos, err
		}
	}

	if err = ctx.Configuration.ReadMap(&m, genieql.MCOPackage(pkg), genieql.MCOType(types.ExprString(determineType(param.Type)))); err != nil {
		return m, infos, err
	}

	infos, _, err = m.MappedColumnInfo(ctx.Driver, ctx.Dialect, ctx.FileSet, pkg)
	return m, infos, err
}

func mappedStructure(ctx Context, param *ast.Field, ignoreSet ...string) ([]genieql.ColumnInfo, []*ast.Field, error) {
	var (
		err     error
		infos   []*ast.Field
		columns []genieql.ColumnInfo
		m       genieql.MappingConfig
		pkg     = ctx.CurrentPackage
	)

	if ipath, err := importPath(ctx, param.Type); err != nil {
		return columns, infos, err
	} else if ipath == ctx.CurrentPackage.ImportPath {
		// when scanning for types we need to reset the build tags to
		// ensure we see the generated code for the other package.
		sbtx := buildx.Clone(ctx.Build, buildx.Tags())
		if pkg, err = astcodec.LocatePackage(ipath, ".", sbtx, genieql.StrictPackageName(ctx.CurrentPackage.Name)); err != nil {
			return columns, infos, err
		}
	}

	if err = ctx.Configuration.ReadMap(&m, genieql.MCOPackage(pkg), genieql.MCOType(types.ExprString(astutil.UnwrapExpr(param.Type)))); err != nil {
		return columns, infos, err
	}

	infos, _, err = m.MapColumnsToFields(
		ctx.FileSet,
		ctx.CurrentPackage,
		genieql.ColumnInfoSet(m.Columns).Filter(genieql.ColumnInfoFilterIgnore(ignoreSet...))...,
	)

	return m.Columns, infos, err
}

func MapFields(ctx Context, params []*ast.Field, ignoreSet ...string) ([]genieql.ColumnMap, error) {
	result := make([]genieql.ColumnMap, 0, len(params))
	for _, param := range params {
		var (
			err     error
			columns []genieql.ColumnMap
		)

		if columns, err = MapField(ctx, param, ignoreSet...); err != nil {
			return result, err
		}

		result = append(result, columns...)
	}

	return result, nil
}

func MapField(ctx Context, param *ast.Field, ignoreSet ...string) ([]genieql.ColumnMap, error) {
	x := removeEllipsis(param.Type)
	if builtinType(x) {
		return builtinParam(ctx, param)
	}

	return mapParam(ctx, param, ignoreSet...)
}

func removeEllipsis(e ast.Expr) ast.Expr {
	if e, ellipsis := e.(*ast.Ellipsis); ellipsis {
		return e.Elt
	}

	return e
}

// mappedParam converts a *ast.Field that represents a struct into an array
// of ColumnMap.
func mapParam(ctx Context, param *ast.Field, ignoreSet ...string) ([]genieql.ColumnMap, error) {
	var (
		err     error
		m       genieql.MappingConfig
		columns []genieql.ColumnInfo
		cMap    []genieql.ColumnMap
	)

	if m, columns, err = mappedParam(ctx, param); err != nil {
		return cMap, err
	}

	aliaser := m.Aliaser()

	for _, arg := range param.Names {
		for _, column := range columns {
			if stringsx.Contains(column.Name, ignoreSet...) {
				continue
			}

			fieldname := transformx.String(column.Name, aliaser)
			cm := column.MapColumn(&ast.SelectorExpr{
				Sel: ast.NewIdent(fieldname),
				X:   arg,
			})
			cm.Field = astutil.Field(ast.NewIdent(column.Definition.Type), ast.NewIdent(fieldname))

			cMap = append(cMap, cm)
		}
	}

	return cMap, nil
}
