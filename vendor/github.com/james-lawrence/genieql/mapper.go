package genieql

import (
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"

	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/internal/errorsx"
	"github.com/james-lawrence/genieql/internal/transformx"
	"golang.org/x/text/transform"
	"gopkg.in/yaml.v3"
)

type _package struct {
	Name       string
	Dir        string
	ImportPath string
}

// MappingConfigOption (MCO) options for building MappingConfigs.
type MappingConfigOption func(*MappingConfig)

// MCOPackage set the package name for the configuration.
func MCOPackage(p *build.Package) MappingConfigOption {
	return func(mc *MappingConfig) {
		mc.Package = _package{
			Name:       p.Name,
			Dir:        p.Dir,
			ImportPath: p.ImportPath,
		}
	}
}

// MCOTransformations specify the transformations to apply to column names.
func MCOTransformations(t ...string) MappingConfigOption {
	return func(mc *MappingConfig) {
		mc.Transformations = t
	}
}

// MCORenameMap rename mapping.
func MCORenameMap(m map[string]string) MappingConfigOption {
	return func(mc *MappingConfig) {
		mc.RenameMap = m
	}
}

// MCOType set the type of the mapping.
func MCOType(t string) MappingConfigOption {
	return func(mc *MappingConfig) {
		mc.Type = t
	}
}

// MCOColumns set the default columns for the mapping.
func MCOColumns(columns ...ColumnInfo) MappingConfigOption {
	return func(mc *MappingConfig) {
		mc.Columns = columns
	}
}

// NewMappingConfig ...
func NewMappingConfig(options ...MappingConfigOption) MappingConfig {
	mc := MappingConfig{}

	(&mc).Apply(options...)

	return mc
}

// MappingConfig TODO...
type MappingConfig struct {
	Package         _package
	Type            string
	Transformations []string
	RenameMap       map[string]string
	Columns         []ColumnInfo
}

// Apply the options to the current MappingConfig
func (t *MappingConfig) Apply(options ...MappingConfigOption) {
	for _, opt := range options {
		opt(t)
	}
}

// Clone the mapping config and apply additional options.
func (t MappingConfig) Clone(options ...MappingConfigOption) MappingConfig {
	for _, opt := range options {
		opt(&t)
	}
	return t
}

// Aliaser ...
func (t MappingConfig) Aliaser() transform.Transformer {
	alias := AliaserBuilder(t.Transformations...)
	return transformx.Full(func(name string) string {
		// if the configuration explicitly renames
		// a column use that value do not try to
		// transform it.
		if v, ok := t.RenameMap[name]; ok {
			return v
		}

		return transformx.String(name, alias)
	})
}

// TypeFields returns the fields of underlying struct of the mapping.
func (t MappingConfig) TypeFields(fset *token.FileSet, pkg *build.Package) ([]*ast.Field, error) {
	return NewSearcher(fset, pkg).FindFieldsForType(ast.NewIdent(t.Type))
}

// MappedColumnInfo returns the mapped and unmapped columns for the mapping.
func (t MappingConfig) MappedColumnInfo(driver Driver, dialect Dialect, fset *token.FileSet, pkg *build.Package) ([]ColumnInfo, []ColumnInfo, error) {
	var (
		err     error
		fields  []*ast.Field
		columns []ColumnInfo
	)

	if fields, err = t.TypeFields(fset, pkg); err != nil {
		return []ColumnInfo(nil), []ColumnInfo(nil), errorsx.Wrapf(err, "failed to lookup fields 0: %s.%s", pkg.Name, t.Type)
	}

	columns = t.Columns
	// if no columns are defined for the mapping lets generate it automatically (may result in incorrect results)
	if len(columns) == 0 {
		log.Println(errorsx.Errorf("no defined columns for: %s.%s generating fake columns", pkg.Name, t.Type))
		// for now just support the CamelCase -> snakecase.
		columns = GenerateFakeColumnInfo(driver, transform.Chain(AliasStrategySnakecase, AliasStrategyLowercase), fields...)
	}

	// returns the sets of mapped and unmapped columns.
	mColumns, uColumns := mapInfo(columns, fields, t.Aliaser())
	return mColumns, uColumns, nil
}

// MappedFields returns the fields that are mapped to columns.
func (t MappingConfig) MappedFields(dialect Dialect, fset *token.FileSet, pkg *build.Package, ignoreColumnSet ...string) ([]*ast.Field, []*ast.Field, error) {
	return t.MapColumnsToFields(
		fset,
		pkg,
		ColumnInfoSet(t.Columns).Filter(ColumnInfoFilterIgnore(ignoreColumnSet...))...,
	)
}

// MapColumns ...
func (t MappingConfig) MapColumns(fset *token.FileSet, pkg *build.Package, local *ast.Ident, columns ...ColumnInfo) (cmap []ColumnMap, umap []ColumnInfo, err error) {
	var (
		fields []*ast.Field
	)

	if fields, err = t.TypeFields(fset, pkg); err != nil {
		return cmap, umap, errorsx.Wrapf(err, "failed to lookup fields 1: %s.%s", t.Package.Name, t.Type)
	}

	mm := func(c ColumnInfo, f *ast.Field) (cmap *ColumnMap) {
		var (
			mapped *ast.Field
		)

		if mapped = MapFieldToNativeType(c, f, t.Aliaser()); mapped == nil {
			return nil
		}

		typex := &ast.SelectorExpr{
			X:   local,
			Sel: astutil.MapFieldsToNameIdent(mapped)[0],
		}

		return &ColumnMap{
			ColumnInfo: c,
			Field:      mapped,
			Dst:        typex,
		}
	}

	cmap, umap = mapColumns(columns, fields, mm)
	return cmap, umap, nil
}

// MapColumnsToFields ...
func (t MappingConfig) MapColumnsToFields(fset *token.FileSet, pkg *build.Package, columns ...ColumnInfo) ([]*ast.Field, []*ast.Field, error) {
	var (
		err    error
		fields []*ast.Field
	)

	if fields, err = t.TypeFields(fset, pkg); err != nil {
		return []*ast.Field{}, []*ast.Field{}, errorsx.Wrapf(err, "failed to lookup fields 2: %s.%s", t.Package.Name, t.Type)
	}

	mFields, uFields := mapFields(columns, fields, func(c ColumnInfo, f *ast.Field) *ast.Field { return MapFieldToNativeType(c, f, t.Aliaser()) })
	return mFields, uFields, nil
}

// MapFieldsToColumns2 ...
func (t MappingConfig) MapFieldsToColumns2(fset *token.FileSet, pkg *build.Package, columns ...ColumnInfo) ([]*ast.Field, []*ast.Field, error) {
	var (
		err    error
		fields []*ast.Field
	)

	if fields, err = t.TypeFields(fset, pkg); err != nil {
		return []*ast.Field{}, []*ast.Field{}, errorsx.Wrapf(err, "failed to lookup fields 3: %s.%s", t.Package.Name, t.Type)
	}

	mFields, uFields := mapFields(columns, fields, func(c ColumnInfo, f *ast.Field) *ast.Field { return MapFieldToSQLType(c, f, t.Aliaser()) })
	return mFields, uFields, nil
}

// WriteMapper persists the structure -> result row mapping to disk.
func WriteMapper(config Configuration, name string, m MappingConfig) error {
	d, err := yaml.Marshal(m)
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp(config.Location, "mkcache.*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err = os.WriteFile(filepath.Join(tmp, name), d, 0666); err != nil {
		return err
	}

	path := filepath.Join(config.Location, filepath.Base(config.Database), m.Package.Name, m.Type, name)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if err = os.Rename(filepath.Join(tmp, name), path); err != nil {
		return err
	}

	return nil
}

// ReadMapper loads the structure -> result row mapping from disk.
func ReadMapper(config Configuration, name string, m *MappingConfig) error {
	var (
		err error
	)

	path := filepath.Join(config.Location, filepath.Base(config.Database), m.Package.Name, m.Type, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(raw, m)
}

// MapFieldToNativeType maps a column to a field based on the provided aliases.
func MapFieldToNativeType(c ColumnInfo, field *ast.Field, aliases ...transform.Transformer) *ast.Field {
	for _, fieldName := range field.Names {
		for _, aliaser := range aliases {
			if transformx.String(c.Name, aliaser) == fieldName.Name {
				return astutil.Field(field.Type, fieldName)
			}
		}
	}
	return nil
}

// MapFieldToSQLType maps a column to a field based on the provided aliases uses the DB type for the field type.
func MapFieldToSQLType(c ColumnInfo, field *ast.Field, aliases ...transform.Transformer) *ast.Field {
	for _, fieldName := range field.Names {
		for _, aliaser := range aliases {
			if transformx.String(c.Name, aliaser) == fieldName.Name {
				return astutil.Field(astutil.MustParseExpr(token.NewFileSet(), c.Definition.ColumnType), fieldName)
			}
		}
	}
	return nil
}

// GenerateFakeColumnInfo generate fake column info from a structure.
func GenerateFakeColumnInfo(d Driver, aliaser transform.Transformer, fields ...*ast.Field) []ColumnInfo {
	results := make([]ColumnInfo, 0, len(fields))
	for _, field := range fields {
		typedef, err := d.LookupType(types.ExprString(field.Type))
		if err != nil {
			log.Println("skipping", astutil.MapExprToString(astutil.MapFieldsToNameExpr(astutil.FlattenFields(field)...)...), "missing type information")
			continue
		}

		for _, name := range field.Names {
			results = append(results, ColumnInfo{
				Name:       transformx.String(name.Name, aliaser),
				Definition: typedef,
			})
		}
	}

	return results
}
